<template>
  <v-app v-if="state === 'success'" class="app taskexec-app">
    <YesNoDialog
      :title="$t('projectRestoreResult')"
      v-model="restoreProjectResultDialog"
      hide-no-button
      :yes-button-title="$t('close')"
      :max-width="400"
    >
      <div class="pt-3" v-if="restoreProjectResult">
        <v-alert dense outlined type="success">
          {{ $t('projectWithNameRestored', { projectName: restoreProjectResult.projectName }) }}
        </v-alert>

        <v-alert dense outlined type="error" class="mb-0">
          <b>{{ $t('emptyKeysRestored', { emptyKeys: restoreProjectResult.emptyKeys }) }}</b>
          {{ $t('pleaseUpdateAccessKeys') }}
        </v-alert>
      </div>
    </YesNoDialog>

    <EditDialog
      v-model="userDialog"
      :save-button-text="$t('save')"
      :title="$t('editUser')"
      v-if="user"
      event-name="i-user"
      :hide-buttons="hideUserDialogButtons"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <UserForm
          :project-id="projectId"
          :item-id="user.id"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
          :is-admin="user.admin"
          :is-self="true"
          :auth-methods="(systemInfo || { auth_methods: {} }).auth_methods"
          :login-with-password="(systemInfo || {}).login_with_password"
          @hide-action-buttons="hideUserDialogButtons = true"
          @show-action-buttons="hideUserDialogButtons = false"
        />
      </template>
    </EditDialog>

    <TaskLogDialog
      v-model="taskLogDialog"
      @close="onTaskLogDialogClosed()"
      :project-id="projectId"
      :item-id="taskId"
      :system-info="systemInfo"
    />

    <EditDialog
      v-model="newProjectDialog"
      :save-button-text="$t('create')"
      :title="$t('newProject')"
      event-name="i-project"
      @close="onNewProjectDialogueClosed()"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <ProjectForm
          v-if="newProjectType === ''"
          item-id="new"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <EditDialog
      v-model="restoreProjectDialog"
      :save-button-text="$t('uiRestore')"
      :title="$t('restoreProject')"
      event-name="i-project"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <RestoreProjectForm
          item-id="new"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <SystemInfoDialog v-model="systemInfoDialog" v-if="user && user.admin" />

    <v-snackbar v-model="snackbar" :color="snackbarColor" :timeout="3000" top>
      {{ snackbarText }}
      <v-btn text @click="snackbar = false">
        {{ $t('close') }}
      </v-btn>
    </v-snackbar>

    <v-navigation-drawer
      app
      :dark="darkMode"
      :light="!darkMode"
      fixed
      width="260"
      v-model="drawer"
      mobile-breakpoint="960"
      :mini-variant="navMini && $vuetify.breakpoint.mdAndUp"
      mini-variant-width="60"
      v-if="showNavigation"
      class="NavDrawer"
      :class="{ 'NavDrawer--canvas': isWorkflowEditor }"
    >
      <router-link
        :to="homeRoute"
        class="taskexec-brand"
        :aria-label="$t('uiTaskExecHome')"
        data-testid="sidebar-home"
      >
        <img src="favicon.svg?v=te-play-30" width="36" height="36" alt="" />
        <span v-if="!navMini">
          <strong>Task<b>Exec</b></strong>
          <small>{{ $t('brandTagline') }}</small>
        </span>
      </router-link>
      <v-menu bottom max-width="235" max-height="100%" v-if="navigationProject">
        <template v-slot:activator="{ on, attrs }">
          <v-list class="pa-0 overflow-y-auto">
            <v-list-item
              key="project"
              class="app__project-selector"
              v-bind="attrs"
              v-on="on"
              data-testid="sidebar-currentProject"
            >
              <v-list-item-icon>
                <ProjectAvatar :project="navigationProject"
                  :color="getProjectColor(navigationProject)" />
              </v-list-item-icon>

              <v-list-item-content>
                <v-list-item-title class="app__project-selector-title">
                  {{ navigationProject.name }}
                </v-list-item-title>
                <v-list-item-subtitle>
                  {{ roleProjectId === navigationProjectId && userRole
                    ? roleTitle(userRole.role) : $t('projectWorkspace') }}
                </v-list-item-subtitle>
              </v-list-item-content>

              <v-list-item-icon>
                <v-icon>mdi-chevron-down</v-icon>
              </v-list-item-icon>
            </v-list-item>
          </v-list>
        </template>
        <v-list>
          <v-list-item
            v-for="(item, i) in projects"
            :key="i"
            :to="`/project/${item.id}`"
            @click="selectProject(item.id)"
          >
            <v-list-item-icon>
              <ProjectAvatar :project="item" :color="getProjectColor(item)" />
            </v-list-item-icon>
            <v-list-item-content>{{ item.name }}</v-list-item-content>
          </v-list-item>

          <v-divider v-if="user.can_create_project" />

          <v-list-item
            @click="showNewProjectDialogue()"
            v-if="user.can_create_project"
            data-testid="sidebar-newProject"
          >
            <v-list-item-icon>
              <v-icon>mdi-plus</v-icon>
            </v-list-item-icon>

            <v-list-item-content>
              {{ $t('newProject2') }}
            </v-list-item-content>
          </v-list-item>

          <v-list-item
            @click="restoreProjectDialog = true"
            v-if="user.can_create_project"
            data-testid="sidebar-restoreProject"
          >
            <v-list-item-icon>
              <v-icon>mdi-backup-restore</v-icon>
            </v-list-item-icon>

            <v-list-item-content>
              {{ $t('restoreProject') }}
            </v-list-item-content>
          </v-list-item>
        </v-list>
      </v-menu>

      <v-list class="pt-0" v-if="!navigationProject && user.can_create_project">
        <v-list-item key="new_project" :to="`/project/new`">
          <v-list-item-icon>
            <v-icon>mdi-plus</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('newProject2') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item key="restore_project" :to="`/project/restore`">
          <v-list-item-icon>
            <v-icon>mdi-restore</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('restoreProject') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>
      </v-list>

      <v-list class="pt-0" v-if="navigationProject">
        <v-list-item
          key="dashboard"
          :to="`/project/${navigationProjectId}/history`"
          data-testid="sidebar-dashboard"
          :class="{ 'taskexec-nav-active': isDashboardPage }"
        >
          <v-list-item-icon>
            <v-icon>mdi-view-dashboard</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ $t('dashboard') }}</v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item
          v-for="item in pinnedNavItemsList"
          :key="item.key"
          :to="item.to"
          :data-testid="item.testId"
          class="nav-item--pinnable"
          :class="{ 'taskexec-nav-active': $route.path.split('/').includes(item.key) }"
        >
          <v-list-item-icon>
            <v-icon>{{ item.icon }}</v-icon>
          </v-list-item-icon>

          <v-list-item-content>
            <v-list-item-title>{{ item.title }}</v-list-item-title>
          </v-list-item-content>

          <div class="nav-pin-wrap" v-if="navEditMode && navItems.length > 1">
            <v-btn icon @click.stop.prevent="togglePin(item.key)" :title="$t('unpin')">
              <v-icon small>mdi-pin-off-outline</v-icon>
            </v-btn>
          </div>
        </v-list-item>

        <template v-if="unpinnedNavItems.length > 0">
          <v-list-item @click="showMoreToggle = !showMoreToggle" class="nav-more-toggle">
            <v-list-item-icon>
              <v-icon>{{ showMoreToggle ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
            </v-list-item-icon>

            <v-list-item-content>
              <v-list-item-title class="nav-more-title">
                {{ $t('more') }}
              </v-list-item-title>
            </v-list-item-content>
          </v-list-item>

          <template v-if="showMoreToggle">
            <v-list-item
              v-for="item in unpinnedNavItems"
              :key="'unpinned-' + item.key"
              :to="item.to"
              :data-testid="item.testId"
              class="nav-item--pinnable"
              :class="{ 'taskexec-nav-active': $route.path.split('/').includes(item.key) }"
            >
              <v-list-item-icon>
                <v-icon>{{ item.icon }}</v-icon>
              </v-list-item-icon>

              <v-list-item-content>
                <v-list-item-title>{{ item.title }}</v-list-item-title>
              </v-list-item-content>

              <div class="nav-pin-wrap" v-if="navEditMode">
                <v-btn icon @click.stop.prevent="togglePin(item.key)" :title="$t('pin')">
                  <v-icon small>mdi-pin-outline</v-icon>
                </v-btn>
              </div>
            </v-list-item>
          </template>
        </template>
      </v-list>

      <template v-slot:append>
        <v-list class="pa-0">
          <div class="NavDrawer__toolsRow">
            <InterfacePreferences
              :dark-mode="darkMode"
              :vertical="navMini && $vuetify.breakpoint.mdAndUp"
              menu-top
              test-id="sidebar"
              @toggle-theme="darkMode = !darkMode"
              @select-language="selectLanguage"
            />

            <span class="NavDrawer__toolsDivider" aria-hidden="true"></span>

            <div class="NavDrawer__actions">
              <v-btn
                icon
                :class="{ 'taskexec-management-active': navEditMode }"
                :title="navEditMode ? $t('finishEditingMenu') : $t('editMenu')"
                :aria-label="navEditMode ? $t('finishEditingMenu') : $t('editMenu')"
                :aria-pressed="String(navEditMode)"
                data-testid="sidebar-edit-menu"
                @click="navEditMode = !navEditMode"
              >
                <v-icon>{{ navEditMode ? 'mdi-check' : 'mdi-playlist-edit' }}</v-icon>
              </v-btn>

              <ManagementMenu
                :is-admin="user.admin"
                :version="(systemInfo || {}).version"
                :active="isManagementPage || !!systemInfoDialog"
                @system-info="systemInfoDialog = true"
              />
            </div>
          </div>

          <v-menu top max-width="235" nudge-top="12">
            <template v-slot:activator="{ on, attrs }">
              <v-list-item
                key="account"
                v-bind="attrs"
                v-on="on"
                :aria-label="$t('editAccount')"
                data-testid="sidebar-account"
                class="NavDrawer__account"
              >
                <v-list-item-icon>
                  <v-icon>mdi-account</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  <v-list-item-title>
                    {{ user.name }}
                  </v-list-item-title>
                </v-list-item-content>

                <v-list-item-action>
                  <v-chip class="taskexec-admin-badge" v-if="user.admin" small>
                    {{ $i18n.t('admin') }}
                  </v-chip>
                </v-list-item-action>
              </v-list-item>
            </template>

            <v-list data-testid="account-menu">
              <v-list-item
                key="edit"
                @click="userDialog = true"
                data-testid="sidebar-edit-account"
              >
                <v-list-item-icon>
                  <v-icon>mdi-pencil</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ $t('editAccount') }}
                </v-list-item-content>
              </v-list-item>

              <v-divider />

              <v-list-item key="sign_out" @click="signOut()" data-testid="sidebar-signout">
                <v-list-item-icon>
                  <v-icon>mdi-exit-to-app</v-icon>
                </v-list-item-icon>

                <v-list-item-content>
                  {{ $t('signOut') }}
                </v-list-item-content>
              </v-list-item>
            </v-list>
          </v-menu>
        </v-list>
      </template>
    </v-navigation-drawer>

    <v-main :class="{ 'taskexec-workspace': showNavigation, 'taskexec-canvas': isWorkflowEditor }">
      <router-view
        :dark-mode="darkMode"
        @toggle-theme="darkMode = !darkMode"
        @select-language="selectLanguage"
        :projectId="projectId"
        :projectType="(project || {}).type || ''"
        :userPermissions="(userRole || {}).permissions"
        :userRole="(userRole || {}).role"
        :userId="(user || {}).id"
        :isAdmin="(user || {}).admin"
        :user="user"
        :features="(systemInfo || { features: {} }).features"
        :authMethods="(systemInfo || { auth_methods: {} }).auth_methods"
        :systemInfo="systemInfo"
      ></router-view>
    </v-main>
  </v-app>
  <v-app v-else-if="state === 'loading'">
    <v-main>
      <v-container fluid fill-height align-center justify-center class="pa-0">
        <v-progress-circular :size="70" color="primary" indeterminate></v-progress-circular>
      </v-container>
    </v-main>
  </v-app>
  <v-app v-else-if="state === 'error'">
    <v-main>
      <v-container
        fluid
        flex-column
        fill-height
        align-center
        justify-center
        class="pa-0 text-center"
      >
        <v-alert text color="error" class="d-inline-block">
          <h3 class="headline">
            {{ $t('error') }}
          </h3>
          {{ snackbarText }}
        </v-alert>
        <div class="mb-6">
          <v-btn text color="blue darken-1" @click="refreshPage()">
            <v-icon left>mdi-refresh</v-icon>
            {{ $t('refreshPage') }}
          </v-btn>
          <v-btn text color="blue darken-1" @click="signOut()">
            <v-icon left>mdi-exit-to-app</v-icon>
            {{ $t('relogin') }}
          </v-btn>
        </div>
      </v-container>
    </v-main>
  </v-app>
  <v-app v-else></v-app>
</template>
<style lang="scss">
.taskexec-brand { display: flex; gap: 12px; align-items: center; padding: 16px 14px;
  color: #fff !important; text-decoration: none; }
.taskexec-brand strong {
  display: block; font-size: 22px; font-weight: 400; letter-spacing: -0.7px;
}
.taskexec-brand b { font-weight: 650; color: var(--taskexec-accent); }
.taskexec-brand small { display: block; color: #b8c8d8; font-size: 11px; }

// Vuetify's reset forces `overflow-y: scroll` on <html>, so every page draws
// an empty scrollbar track on the right. Show the page scrollbar only when
// the content actually overflows. `:root` outranks Vuetify's `html` selector,
// whose stylesheet is injected after this one.
html:root {
  overflow-y: auto;
}

// Set by WorkflowEditor while mounted: the editor is exactly viewport-high,
// so the page scrollbar Vuetify forces on <html> would only draw an empty track.
html.WorkflowEditor-html {
  overflow-y: hidden;
}

// The page itself never scrolls in the editor, so any trackpad gesture the
// canvas or a side panel does not take ends up as viewport overscroll, which
// Chrome on macOS turns into Back / Forward navigation. Chrome only does that
// when the viewport's overscroll-behavior propagates; it reads the value from
// <html> (older versions: from the viewport-defining element, which can be
// <body>), so set both.
html.WorkflowEditor-html,
html.WorkflowEditor-html body {
  overscroll-behavior: none;
}

.NavDrawer {
  height: 100dvh !important;
  // No width animation: the drawer switches between full and icon-only
  // together with a route change and must take its final width at once.
  transition-property: transform, visibility;

  // Icon-only strip while the workflow editor has its palette collapsed.
  // The global first-child icon rule below is !important, so it has to be
  // undone here with the same weight.
  &.v-navigation-drawer--mini-variant {
    .v-list-item__icon:first-child {
      margin-right: 0 !important;
    }
  }
}

// Vuetify animates the main area's padding when the drawer width changes;
// the drawer width itself is not animated (see .NavDrawer), so neither is this.
.v-main {
  transition: none;
}

.nav-item--pinnable {
  .nav-pin-wrap {
    margin-left: auto;
    display: flex;
    align-items: center;
  }
}

.nav-more-toggle {
  min-height: 36px;

  .nav-more-title {
    opacity: 0.6;
  }
}

.NestedDialog {
  border: 1px solid rgb(200, 200, 200);
  border-radius: 12px;
}

.theme--dark {
  --highlighted-card-bg-color: var(--taskexec-soft, #20333d);

  // Dialogs opened above another dialog have no overlay; the default Vuetify
  // shadow is too faint on a dark background, so use a wider, denser one.
  .NestedDialog {
    border: 1px solid rgb(80, 80, 80);
    box-shadow: 0 30px 80px 16px rgb(10, 10, 10);
  }
}

.theme--light {
  --highlighted-card-bg-color: var(--taskexec-soft, #f0f5f4);
}

.v-dialog > .v-card > .v-card__title {
  flex-wrap: nowrap;
  overflow: hidden;
}

.v-data-table tbody tr.v-data-table__expanded__content {
  box-shadow: none !important;
}

.v-data-table a {
  text-decoration-line: none;

  &:hover {
    text-decoration-line: underline;
  }
}

.breadcrumbs__item--link {
  text-decoration-line: none;

  &:hover {
    text-decoration-line: underline;
  }
}

.breadcrumbs__separator {
  padding: 0 10px;
}

.app__project-selector {
  height: 64px;

  & > .v-list-item__content {
    padding: 0;
  }

  .v-list-item__icon {
    margin-top: 20px !important;
  }
}

.app__project-selector-title {
  font-size: 1.25rem !important;
  font-weight: bold;
}

.v-application--is-ltr .v-list-item__action:first-child,
.v-application--is-ltr .v-list-item__icon:first-child {
  margin-right: 16px !important;
}

.v-toolbar__content {
  height: 64px !important;
}

.v-data-table .v-data-footer {
  margin-left: 16px !important;
  margin-right: 16px !important;
}

.v-data-table__wrapper {
  padding-left: 16px !important;
  padding-right: 16px !important;
}

.v-data-table {
  td:first-child,
  th:first-child {
    padding-left: 2px !important;
  }

  td:last-child,
  th:last-child {
    padding-right: 2px !important;
  }

  .v-data-table__wrapper > table > thead > tr:last-child > th {
    text-transform: uppercase;
    white-space: nowrap;
  }

  .v-data-table__wrapper > table > tbody > tr {
    background: transparent !important;

    &:hover {
      background-color: rgba(143, 143, 143, 0.04) !important;
    }

    & > td {
      white-space: nowrap;
    }
  }
}

.v-data-table > .v-data-table__wrapper > table > tbody > tr > th,
.v-data-table > .v-data-table__wrapper > table > thead > tr > th,
.v-data-table > .v-data-table__wrapper > table > tfoot > tr > th,
.v-data-table > .v-data-table__wrapper > table > tbody > tr > td {
  font-size: 1rem !important;
}

.v-data-footer {
  font-size: 1rem !important;
}

.v-toolbar__title {
  font-weight: bold !important;
}

.v-app-bar__nav-icon {
  margin-left: 0 !important;
}

.v-toolbar__title:not(:first-child) {
  margin-left: 10px !important;
}

.v-slide-group__prev--disabled {
  display: none !important;
}

@media (min-width: 960px) {
  .v-app-bar__nav-icon {
    display: none !important;
  }

  .v-toolbar__title:not(:first-child) {
    padding-left: 0 !important;
    margin-left: 0 !important;
  }
}

.v-input {
  .v-input__slot fieldset {
    border-radius: 8px;
    border-width: 1px;
    border-color: rgba(133, 133, 133, 0.4);
    background-color: rgba(133, 133, 133, 0.1);
  }

  .v-label--active {
    text-shadow: 0 0 2px black;
    font-weight: 500;
  }

  &.primary--text {
    .v-input__slot fieldset {
      border-width: 2px;
      border-color: #2196f3;
    }
  }

  &.error--text {
    .v-input__slot fieldset {
      border-width: 2px;
      border-color: #ff5252;
    }
  }
}

.v-input--is-disabled {
  opacity: 0.5;
}

.theme--light {
  .v-input {
    .v-label--active {
      text-shadow: 0 0 2px white;
    }
  }
}

.v-list--dense .v-list-item .v-list-item__title {
  font-weight: normal;
  font-size: 1rem;
}

@import '~vuetify/src/styles/styles.sass';
@media #{map-get($display-breakpoints, 'xl-only')} {
  .CenterToScreen {
    transform: translateX(-130px);
  }
}
</style>

<script>
import DisplayLabelsMixin from '@/components/DisplayLabelsMixin';
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import EditDialog from '@/components/EditDialog.vue';
import ProjectForm from '@/components/ProjectForm.vue';
import ProjectAvatar from '@/components/ProjectAvatar.vue';
import UserForm from '@/components/UserForm.vue';
import EventBus from '@/event-bus';
import { isWorkflowEditorPath, navMiniForPath } from '@/lib/workflowEditorPrefs';
import socket from '@/socket';

import RestoreProjectForm from '@/components/RestoreProjectForm.vue';
import YesNoDialog from '@/components/YesNoDialog.vue';
import TaskLogDialog from '@/components/TaskLogDialog.vue';
import SystemInfoDialog from '@/components/SystemInfoDialog.vue';
import ManagementMenu from '@/components/ManagementMenu.vue';
import InterfacePreferences from '@/components/InterfacePreferences.vue';
import delay from '@/lib/delay';
import { normalizeLocale } from '@/lib/locale';

const PROJECT_COLORS = ['red', 'blue', 'orange', 'green'];

export default {
  mixins: [DisplayLabelsMixin],
  name: 'App',
  components: {

    TaskLogDialog,
    YesNoDialog,
    RestoreProjectForm,
    UserForm,
    EditDialog,
    ProjectForm,
    ProjectAvatar,
    SystemInfoDialog,
    ManagementMenu,
    InterfacePreferences,
  },
  data() {
    return {
      drawer: null,
      // Icon-only navigation while the workflow editor has its palette
      // collapsed. Derived from the route up front so the drawer renders in
      // its final width; the editor then keeps it in sync via i-nav-mini.
      navMini: navMiniForPath(this.$route.path),
      user: null,
      userRole: null,
      roleProjectId: null,
      lastProjectId: parseInt(localStorage.getItem('projectId'), 10) || null,
      systemInfo: null,
      state: 'loading',
      snackbar: false,
      snackbarText: '',
      snackbarColor: '',
      projects: null,
      newProjectDialog: null,
      newProjectType: '',
      userDialog: null,
      hideUserDialogButtons: false,

      systemInfoDialog: null,

      restoreProjectDialog: null,
      restoreProjectResult: null,
      restoreProjectResultDialog: null,

      taskLogDialog: null,
      taskId: null,
      template: null,
      darkMode: this.$vuetify.theme.dark,
      unpinnedNavKeys: [],
      showMoreToggle: false,
      navEditMode: false,
    };
  },

  watch: {
    async projects(val) {
      if (
        val.length === 0
        && this.$route.path.startsWith('/project/')
        && this.$route.path !== '/project/new'
      ) {
        await this.$router.push({ path: '/project/new' });
      }
    },

    async $route(val) {
      this.navMini = navMiniForPath(val.path);

      if (this.state === 'success' && this.user && this.projects) {
        try {
          // App stays mounted during client navigation, so home must resolve
          // here as well as during the initial load.
          if (val.path === '/' || val.path === '/project') {
            await this.trySelectMostSuitableProject();
            return;
          }
          if (this.project && this.roleProjectId !== this.projectId) {
            await this.selectProject(this.projectId);
          }
        } catch (err) {
          EventBus.$emit('i-snackbar', { color: 'error', text: getErrorMessage(err) });
        }
      }

      if (val.query.t == null) {
        this.taskLogDialog = false;
      } else {
        const taskId = parseInt(this.$route.query.t || '', 10);
        if (taskId) {
          EventBus.$emit('i-show-task', { taskId });
        }
      }

      if ((this.projects || []).length > 0 && this.$route.query.new_project) {
        EventBus.$emit('i-new-project', { projectType: this.$route.query.new_project });
      }

      if (this.unpinnedNavItems.some((item) => val.path.includes(`/${item.key}`))) {
        this.showMoreToggle = true;
      }
    },

    darkMode: {
      immediate: true,
      handler(val) {
        this.$vuetify.theme.dark = val;
        document.documentElement.style.colorScheme = val ? 'dark' : 'light';
        if (val) {
          localStorage.setItem('darkMode', '1');
        } else {
          localStorage.removeItem('darkMode');
        }
      },
    },
  },

  computed: {
    showNavigation() {
      return !!this.user && this.$route.path !== '/auth/login'
        && !this.$route.path.startsWith('/accept-invite/');
    },

    navigationProject() {
      const projects = this.projects || [];
      return this.project || projects.find((p) => p.id === this.lastProjectId) || projects[0];
    },

    navigationProjectId() {
      return (this.navigationProject || {}).id || null;
    },

    homeRoute() {
      return this.navigationProjectId ? `/project/${this.navigationProjectId}/history` : '/project/new';
    },

    isManagementPage() {
      return ['/users', '/runners', '/tasks', '/apps', '/roles', '/tokens', '/alerts', '/settings']
        .includes(this.$route.path);
    },

    isDashboardPage() {
      return /^\/project\/\d+\/(history|stats|activity|settings)\/?$/.test(this.$route.path);
    },

    isWorkflowEditor() {
      return isWorkflowEditorPath(this.$route.path);
    },

    projectId() {
      return parseInt(this.$route.params.projectId, 10) || null;
    },

    project() {
      if (this.projects == null) {
        return null;
      }
      return this.projects.find((x) => x.id === this.projectId);
    },

    templatesUrl() {
      let viewId = localStorage.getItem(`project${this.navigationProjectId}__lastVisitedViewId`);
      if (viewId) {
        viewId = parseInt(viewId, 10);
        if (!Number.isNaN(viewId)) {
          return `/project/${this.navigationProjectId}/views/${viewId}/templates`;
        }
      }
      return `/project/${this.navigationProjectId}/templates`;
    },

    navItems() {
      if (!this.navigationProject) return [];
      const base = `/project/${this.navigationProjectId}`;
      const items = [];

      if (this.navigationProject.type === '') {
        items.push(
          {
            key: 'templates',
            icon: 'mdi-check-all',
            title: this.$t('taskTemplates'),
            to: this.templatesUrl,
            testId: 'sidebar-templates',
          },
          {
            key: 'workflows',
            icon: 'mdi-graph-outline',
            title: this.$t('workflows'),
            to: `${base}/workflows`,
            testId: 'sidebar-workflows',
          },
          {
            key: 'schedule',
            icon: 'mdi-clock-outline',
            title: this.$t('schedule'),
            to: `${base}/schedule`,
            testId: 'sidebar-schedule',
          },
          {
            key: 'inventory',
            icon: 'mdi-monitor-multiple',
            title: this.$t('inventory'),
            to: `${base}/inventory`,
            testId: 'sidebar-inventory',
          },
          {
            key: 'environment',
            icon: 'mdi-code-braces',
            title: this.$t('environment'),
            to: `${base}/environment`,
            testId: 'sidebar-environment',
          },
          {
            key: 'keys',
            icon: 'mdi-key-change',
            title: this.$t('keyStore'),
            to: `${base}/keys`,
            testId: 'sidebar-keys',
          },
          {
            key: 'repositories',
            icon: 'mdi-git',
            title: this.$t('repositories'),
            to: `${base}/repositories`,
          },
          {
            key: 'host_config',
            icon: 'mdi-server-network',
            title: this.$t('hostConfig'),
            to: `${base}/host_config`,
          },
          {
            key: 'integrations',
            icon: 'mdi-connection',
            title: this.$t('integrations'),
            to: `${base}/integrations`,
            testId: 'sidebar-integrations',
          },
        );
      }

      items.push({
        key: 'team',
        icon: 'mdi-account-multiple',
        title: this.$t('team'),
        to: `${base}/team`,
        testId: 'sidebar-team',
      });

      if (this.systemInfo?.features?.project_runners && this.navigationProject.type === '') {
        items.push({
          key: 'runners',
          icon: 'mdi-cogs',
          title: this.$t('runners'),
          to: `${base}/runners`,
          testId: 'sidebar-runners',
        });
      }

      // Show only implemented capabilities returned by the server.
      const features = (this.systemInfo || {}).features || {};
      return items.filter((it) => it.key !== 'workflows' || features.workflows);
    },

    pinnedNavItemsList() {
      return this.navItems.filter((item) => !this.unpinnedNavKeys.includes(item.key));
    },

    unpinnedNavItems() {
      return this.navItems.filter((item) => this.unpinnedNavKeys.includes(item.key));
    },
  },

  async created() {
    try {
      await this.loadData();
      this.state = 'success';
    } catch (err) {
      if (err.response && err.response.status === 401) {
        if (this.$route.path !== '/auth/login') {
          await this.$router.replace({
            path: '/auth/login',
            query: { return: this.$route.fullPath },
          });
        }
        this.state = 'success';
        return;
      }

      EventBus.$emit('i-snackbar', {
        color: 'error',
        text: getErrorMessage(err),
      });
      this.state = 'error';
      socket.stop();
    }
  },

  mounted() {
    EventBus.$on('i-server-settings', (settings) => {
      if (this.systemInfo) {
        this.systemInfo.use_remote_runner = settings.use_remote_runner;
      }
    });

    EventBus.$on('i-snackbar', (e) => {
      this.snackbar = true;
      this.snackbarColor = e.color;
      this.snackbarText = e.text;
    });

    EventBus.$on('i-account-change', async () => {
      await this.loadUserInfo();
    });

    EventBus.$on('i-show-drawer', async () => {
      this.drawer = true;
    });

    EventBus.$on('i-nav-mini', (e) => {
      this.navMini = !!(e && e.mini);
    });

    EventBus.$on('i-new-project', (e) => {
      setTimeout(() => {
        this.showNewProjectDialogue(e.projectType);
      }, 500);
    });

    EventBus.$on('i-show-task', async (e) => {
      if (parseInt(this.$route.query.t || '', 10) !== e.taskId) {
        const query = { ...this.$route.query, t: e.taskId };
        await this.$router.replace({ query });
        return;
      }

      this.taskId = e.taskId;
      await delay(1);
      this.taskLogDialog = true;
    });

    EventBus.$on('i-open-last-project', async () => {
      await this.trySelectMostSuitableProject();
    });

    EventBus.$on('i-user', async (e) => {
      let text;

      switch (e.action) {
        case 'new':
          text = this.$t('userCreated', { name: e.item.name });
          break;
        case 'edit':
          text = this.$t('userSaved', { name: e.item.name });
          break;
        case 'delete':
          text = this.$t('userDeleted', { name: e.item.name });
          break;
        default:
          throw new Error('Unknown project action');
      }

      EventBus.$emit('i-snackbar', {
        color: 'success',
        text,
      });

      if (this.user && e.item.id === this.user.id) {
        await this.loadUserInfo();
      }
    });

    EventBus.$on('i-project', async (e) => {
      let text;

      const project = this.projects.find((p) => p.id === e.item.id) || e.item;
      const projectName = project.name || `#${project.id}`;

      switch (e.action) {
        case 'new':
          text = this.$t('projectCreated', { name: projectName });
          break;
        case 'edit':
          text = this.$t('projectSaved', { name: projectName });
          break;
        case 'delete':
          text = this.$t('projectDeleted', { name: projectName });
          break;
        case 'restore':
          break;
        default:
          throw new Error('Unknown project action');
      }

      if (e.action === 'restore') {
        const emptyKeys = (
          await axios({
            method: 'get',
            url: `/api/project/${project.id}/keys`,
            responseType: 'json',
          })
        ).data.filter((k) => k.empty);

        this.restoreProjectResult = {
          projectName,
          emptyKeys: emptyKeys.length,
        };
        this.restoreProjectResultDialog = true;
      } else {
        EventBus.$emit('i-snackbar', {
          color: 'success',
          text,
        });
      }

      await this.loadProjects();

      switch (e.action) {
        case 'new':
        case 'restore':
          await this.selectProject(e.item.id, { new_project: undefined });
          break;
        case 'delete':
          if (this.projectId === e.item.id && this.projects.length > 0) {
            await this.selectProject(this.projects[0].id);
          }
          break;
        default:
          break;
      }
    });
  },

  methods: {
    showNewProjectDialogue(projectType = '') {
      this.newProjectDialog = true;
      this.newProjectType = projectType;
    },

    async togglePin(key) {
      if (this.unpinnedNavKeys.includes(key)) {
        this.unpinnedNavKeys = this.unpinnedNavKeys.filter((k) => k !== key);
      } else {
        this.unpinnedNavKeys = [...this.unpinnedNavKeys, key];
      }
      await this.saveUnpinnedNavKeys();
    },

    async saveUnpinnedNavKeys() {
      try {
        await axios({
          method: 'post',
          url: '/api/user/options',
          responseType: 'json',
          data: { key: 'nav.unpinnedItems', value: JSON.stringify(this.unpinnedNavKeys) },
        });
      } catch (err) {
        EventBus.$emit('i-snackbar', { color: 'error', text: getErrorMessage(err) });
      }
    },

    applyLanguage(lang) {
      if (typeof lang !== 'string' || lang === '') {
        localStorage.removeItem('lang');
        this.$i18n.locale = normalizeLocale(navigator.language);
        return;
      }

      const locale = normalizeLocale(lang);
      localStorage.setItem('lang', locale);
      this.$i18n.locale = locale;
    },

    async loadUserOptions() {
      const options = (
        await axios({
          method: 'get',
          url: '/api/user/options',
          responseType: 'json',
        })
      ).data;

      if (options['nav.unpinnedItems'] != null) {
        try {
          this.unpinnedNavKeys = JSON.parse(options['nav.unpinnedItems']);
        } catch {
          // do nothing
        }
      }

      if (options.lang != null) {
        const currentLang = localStorage.getItem('lang');
        try {
          this.applyLanguage(JSON.parse(options.lang));
        } catch {
          this.applyLanguage(currentLang);
        }
      }
    },

    async selectLanguage(lang) {
      const previousLang = localStorage.getItem('lang');
      this.applyLanguage(lang);

      // Keep the current login/verification form intact for anonymous visitors.
      if (this.$route.path === '/auth/login') return;

      if (this.user) {
        try {
          await axios({
            method: 'post',
            url: '/api/user/options',
            responseType: 'json',
            data: { key: 'lang', value: JSON.stringify(lang) },
          });
        } catch (err) {
          this.applyLanguage(previousLang);
          EventBus.$emit('i-snackbar', { color: 'error', text: getErrorMessage(err) });
          return;
        }
      }

      window.location.reload();
    },

    async onNewProjectDialogueClosed() {
      const query = { ...this.$route.query, new_project: undefined };
      await this.$router.replace({ query });
    },

    async onTaskLogDialogClosed() {
      const query = { ...this.$route.query, t: undefined };
      await this.$router.replace({ query });
    },

    async loadData() {
      await this.loadUserInfo();
      await this.loadUserOptions();

      // Activate session and start socket only after confirming user is authenticated
      socket.setSessionActive(true);
      if (!socket.isRunning()) {
        socket.start();
      }

      await this.loadProjects();

      // try to find project and switch to it if URL not pointing to any project
      if (
        this.$route.path === '/'
        || this.$route.path === '/project'
        || this.$route.path.startsWith('/project/')
      ) {
        await this.trySelectMostSuitableProject();
      }

      // display task dialog if query param t specified
      if (this.$route.query.t) {
        const taskId = parseInt(this.$route.query.t || '', 10);
        if (taskId) {
          EventBus.$emit('i-show-task', { taskId });
        }
      }

      if ((this.projects || []).length > 0 && this.$route.query.new_project != null) {
        EventBus.$emit('i-new-project', { projectType: this.$route.query.new_project });
      }
    },

    async trySelectMostSuitableProject() {
      if (this.projects.length === 0) {
        if (this.$route.path !== '/project/new') {
          await this.$router.push({ path: '/project/new' });
        }
        return;
      }

      let projectId;

      if (this.projectId) {
        projectId = this.projectId;
      }

      if (
        (projectId == null || !this.projects.some((p) => p.id === projectId))
        && localStorage.getItem('projectId')
      ) {
        projectId = parseInt(localStorage.getItem('projectId'), 10);
      }

      if (projectId == null || !this.projects.some((p) => p.id === projectId)) {
        projectId = this.projects[0].id;
      }

      if (projectId != null) {
        await this.selectProject(projectId);
      }
    },

    async selectProject(projectId, overriderQuery = {}) {
      this.userRole = (
        await axios({
          method: 'get',
          url: `/api/project/${projectId}/role`,
          responseType: 'json',
        })
      ).data;

      localStorage.setItem('projectId', projectId);
      this.lastProjectId = projectId;
      this.roleProjectId = projectId;
      if (this.projectId === projectId) {
        return;
      }

      let query = {};

      switch (this.$route.path) {
        case '/project/new':
          query.new_project = '';
          break;
        default:
          break;
      }

      query = {
        ...query,
        ...overriderQuery,
      };

      await this.$router.push({
        path: `/project/${projectId}${window.location.search}`,
        query,
      });
    },

    async loadProjects() {
      this.projects = (
        await axios({
          method: 'get',
          url: '/api/projects',
          responseType: 'json',
        })
      ).data;
    },

    async loadUserInfo() {
      this.user = (
        await axios({
          method: 'get',
          url: '/api/user',
          responseType: 'json',
        })
      ).data;

      this.systemInfo = (
        await axios({
          method: 'get',
          url: '/api/info',
          responseType: 'json',
        })
      ).data;
    },

    getProjectColor(projectData) {
      const i = this.projects.length - this.projects.findIndex((p) => p.id === projectData.id);
      return PROJECT_COLORS[i % PROJECT_COLORS.length];
    },

    async restoreProject() {
      const f = document.createElement('input');
      f.setAttribute('type', 'file');
      f.addEventListener('change', (e) => {
        const file = e.target.files[0];
        if (file) {
          const reader = new FileReader();
          reader.onload = async (ev) => {
            const fileContent = ev.target.result;
            try {
              await axios.post('/api/projects/restore', fileContent).then(async (payload) => {
                this.$router.push({ path: `/project/${payload.data.id}/history` });
                this.state = 'success';
                await this.loadProjects();
              });
            } catch (err) {
              EventBus.$emit('i-snackbar', {
                color: 'error',
                text: getErrorMessage(err),
              });
            }
          };
          reader.readAsText(file);
        }
      });
      f.click();
    },

    async signOut() {
      this.snackbar = false;
      this.snackbarColor = '';
      this.snackbarText = '';

      try {
        await axios({
          method: 'post',
          url: '/api/auth/logout',
          responseType: 'json',
        });

        socket.setSessionActive(false);
        socket.stop();
        this.user = null;
        this.userRole = null;
        this.roleProjectId = null;

        if (this.$route.path !== '/auth/login') {
          await this.$router.push({ path: '/auth/login' });
          this.state = 'success';
        }
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }
    },

    refreshPage() {
      const { location } = document;
      document.location = location;
    },
  },
};
</script>
