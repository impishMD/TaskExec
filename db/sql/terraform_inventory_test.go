package sql

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func terraformStoreFixture(t *testing.T) (*SqlDb, db.Project, db.Inventory, db.AccessKey) {
	t.Helper()
	previous := util.Config
	store := InitConfigCreateTestStore()
	// Opt-in integration checks against disposable SQL servers, using the same
	// migrations and store implementation as production. Never uses app DB envs.
	if dialect := os.Getenv("TASKEXEC_TEST_TERRAFORM_DIALECT"); dialect != "" {
		require.Contains(t, []string{util.DbDriverPostgres, util.DbDriverMySQL}, dialect)
		store.Close()
		config := &util.DbConfig{Hostname: os.Getenv("TASKEXEC_TEST_TERRAFORM_HOST"), Username: "taskexec", Password: os.Getenv("TASKEXEC_TEST_TERRAFORM_PASSWORD"), DbName: "taskexec_terraform_qa", Options: map[string]string{}}
		util.Config.Dialect = dialect
		if dialect == util.DbDriverPostgres {
			config.Options["sslmode"] = "disable"
			util.Config.Postgres = config
		} else {
			util.Config.MySQL = config
		}
		store = CreateDb(dialect)
		store.Connect()
		require.NoError(t, db.Migrate(store, nil))
	}
	t.Cleanup(func() { store.Close(); util.Config = previous })
	p, err := store.CreateProject(db.Project{Name: "Terraform"})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := store.DeleteProject(p.ID); err != nil && !errors.Is(err, db.ErrNotFound) {
			t.Errorf("remove Terraform test project: %v", err)
		}
	})
	inv, err := store.CreateInventory(db.Inventory{ProjectID: p.ID, Name: "default", Inventory: "default", Type: db.InventoryTofuWorkspace})
	require.NoError(t, err)
	key, err := store.CreateAccessKey(db.AccessKey{Name: "backend", ProjectID: &p.ID, Type: db.AccessKeyLoginPassword})
	require.NoError(t, err)
	return store, p, inv, key
}

func TestTerraformMigrationPreservesHistoricalState(t *testing.T) {
	version := "2.20.12"
	store := InitConfigCreateTestStoreAt(&version)
	t.Cleanup(store.Close)
	p, err := createLegacyTestProject(store, db.Project{Name: "legacy Terraform"})
	require.NoError(t, err)
	inv, err := store.CreateInventory(db.Inventory{ProjectID: p.ID, Name: "legacy", Type: db.InventoryTerraformWorkspace})
	require.NoError(t, err)
	keyID, err := store.insert("id", "insert into access_key (project_id, name, type) values (?, ?, ?)", p.ID, "fixture", db.AccessKeyLoginPassword)
	key := db.AccessKey{ID: keyID}
	require.NoError(t, err)
	_, err = store.exec("insert into project__terraform_inventory_alias (alias,project_id,inventory_id,auth_key_id) values (?,?,?,?)", "legacy-alias", p.ID, inv.ID, key.ID)
	require.NoError(t, err)
	_, err = store.exec("insert into project__terraform_inventory_state (project_id,inventory_id,state,created) values (?,?,?,?)", p.ID, inv.ID, `{"version":4,"serial":42}`, time.Now())
	require.NoError(t, err)
	require.NoError(t, db.Migrate(store, nil))
	require.NoError(t, db.Migrate(store, nil))
	state, err := store.GetLatestTerraformInventoryState(p.ID, inv.ID)
	require.NoError(t, err)
	require.JSONEq(t, `{"version":4,"serial":42}`, state.State)
	alias, err := store.GetTerraformInventoryAliasByAlias("legacy-alias")
	require.NoError(t, err)
	require.Equal(t, key.ID, alias.AuthKeyID)
	require.NoError(t, store.LockTerraformInventoryState(p.ID, inv.ID, db.TerraformStateLock{ID: "new-lock"}))
}
func TestTerraformStoreHistoryAndLocks(t *testing.T) {
	store, p, inv, key := terraformStoreFixture(t)
	baseline, err := store.GetTerraformStateCount()
	require.NoError(t, err)
	alias, err := store.CreateTerraformInventoryAlias(db.TerraformInventoryAlias{ProjectID: p.ID, InventoryID: inv.ID, AuthKeyID: key.ID, Alias: fmt.Sprintf("first-%d", p.ID)})
	require.NoError(t, err)
	other := alias
	other.Alias = fmt.Sprintf("second-%d", p.ID)
	_, err = store.CreateTerraformInventoryAlias(other)
	require.NoError(t, err)
	refs, err := store.GetAccessKeyRefs(p.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, []db.ObjectReferrer{{ID: inv.ID, Name: inv.Name}}, refs.Inventories)
	require.Error(t, store.DeleteAccessKey(p.ID, key.ID), "a backend credential cannot be deleted while referenced")
	lock := db.TerraformStateLock{ID: "lock-one", Operation: "apply", Who: "test"}
	require.NoError(t, store.LockTerraformInventoryState(p.ID, inv.ID, lock))
	require.NoError(t, store.LockTerraformInventoryState(p.ID, inv.ID, lock))
	err = store.LockTerraformInventoryState(p.ID, inv.ID, db.TerraformStateLock{ID: "lock-two"})
	var conflict *db.TerraformLockError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, lock, conflict.Lock)
	state := db.TerraformInventoryState{ProjectID: p.ID, InventoryID: inv.ID, State: `{"version":4,"serial":1,"outputs":{"password":{"value":"private-state"}}}`}
	_, err = store.SaveTerraformInventoryState(state, "")
	require.ErrorAs(t, err, &conflict)
	_, err = store.SaveTerraformInventoryState(state, "wrong")
	require.ErrorAs(t, err, &conflict)
	first, err := store.SaveTerraformInventoryState(state, lock.ID)
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	state.State = `{"version":4,"serial":2}`
	second, err := store.SaveTerraformInventoryState(state, lock.ID)
	require.NoError(t, err)
	latest, err := store.GetLatestTerraformInventoryState(p.ID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, latest.ID)
	states, err := store.GetTerraformInventoryStates(p.ID, inv.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, states, 2)
	require.Empty(t, states[0].State)
	require.ErrorAs(t, store.DeleteTerraformInventoryState(p.ID, inv.ID, first.ID), &conflict)
	require.ErrorAs(t, store.UnlockTerraformInventoryState(p.ID, inv.ID, "wrong"), &conflict)
	require.NoError(t, store.ResetTerraformInventoryState(p.ID, inv.ID, lock.ID))
	_, err = store.GetLatestTerraformInventoryState(p.ID, inv.ID)
	require.ErrorIs(t, err, db.ErrNotFound)
	historical, err := store.GetTerraformInventoryState(p.ID, inv.ID, first.ID)
	require.NoError(t, err)
	require.Contains(t, historical.State, "private-state")
	require.NoError(t, store.UnlockTerraformInventoryState(p.ID, inv.ID, lock.ID))
	require.NoError(t, store.UnlockTerraformInventoryState(p.ID, inv.ID, lock.ID))
	_, err = store.SaveTerraformInventoryState(state, lock.ID)
	require.ErrorAs(t, err, &conflict, "stale lock IDs cannot write")
	third, err := store.SaveTerraformInventoryState(state, "")
	require.NoError(t, err)
	require.NoError(t, store.DeleteTerraformInventoryState(p.ID, inv.ID, third.ID))
	_, err = store.GetLatestTerraformInventoryState(p.ID, inv.ID)
	require.ErrorIs(t, err, db.ErrNotFound, "deleting latest must never resurrect an older state")
	require.NoError(t, store.DeleteTerraformInventoryState(p.ID, inv.ID, first.ID))
	states, err = store.GetTerraformInventoryStates(p.ID, inv.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.Equal(t, second.ID, states[0].ID)
	count, err := store.GetTerraformStateCount()
	require.NoError(t, err)
	require.Equal(t, baseline+1, count)
}
func TestTerraformStoreScopeAndValidation(t *testing.T) {
	store, p, inv, key := terraformStoreFixture(t)
	foreign, err := store.CreateProject(db.Project{Name: "foreign"})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.DeleteProject(foreign.ID)) })
	foreignInv, err := store.CreateInventory(db.Inventory{ProjectID: foreign.ID, Name: "foreign", Type: db.InventoryTerraformWorkspace})
	require.NoError(t, err)
	other, err := store.CreateAccessKey(db.AccessKey{ProjectID: &foreign.ID, Name: "foreign", Type: db.AccessKeyLoginPassword})
	require.NoError(t, err)
	for _, alias := range []db.TerraformInventoryAlias{
		{ProjectID: p.ID, InventoryID: foreignInv.ID, AuthKeyID: key.ID, Alias: "foreign-inventory"},
		{ProjectID: p.ID, InventoryID: inv.ID, AuthKeyID: other.ID, Alias: "foreign-key"},
	} {
		_, err = store.CreateTerraformInventoryAlias(alias)
		require.Error(t, err)
	}
	for _, raw := range []string{"", `null`, `[]`, `{invalid`} {
		_, err = store.CreateTerraformInventoryState(db.TerraformInventoryState{ProjectID: p.ID, InventoryID: inv.ID, State: raw})
		require.Error(t, err)
	}
	state, err := store.CreateTerraformInventoryState(db.TerraformInventoryState{ProjectID: p.ID, InventoryID: inv.ID, State: `{"version":4}`})
	require.NoError(t, err)
	_, err = store.GetTerraformInventoryState(foreign.ID, inv.ID, state.ID)
	require.ErrorIs(t, err, db.ErrNotFound)
	_, err = store.GetTerraformInventoryState(p.ID, foreignInv.ID, state.ID)
	require.ErrorIs(t, err, db.ErrNotFound)
	require.ErrorIs(t, store.DeleteTerraformInventoryState(foreign.ID, inv.ID, state.ID), db.ErrNotFound)
	require.ErrorIs(t, store.LockTerraformInventoryState(p.ID, foreignInv.ID, db.TerraformStateLock{ID: "bad"}), db.ErrNotFound)
	require.NoError(t, store.LockTerraformInventoryState(p.ID, inv.ID, db.TerraformStateLock{ID: "keep"}))
	require.NoError(t, store.DeleteProject(p.ID))
	var count int
	err = store.selectOne(&count, "select count(*) from project__terraform_inventory_lock where inventory_id=?", inv.ID)
	require.NoError(t, err)
	require.Zero(t, count)
}
func TestTerraformStoreConcurrentLock(t *testing.T) {
	store, p, inv, _ := terraformStoreFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Go(func() {
			results <- store.LockTerraformInventoryState(p.ID, inv.ID, db.TerraformStateLock{ID: fmt.Sprintf("lock-%d", i)})
		})
	}
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
			continue
		}
		var conflict *db.TerraformLockError
		require.True(t, errors.As(err, &conflict), "%v", err)
	}
	require.Equal(t, 1, winners)
}
