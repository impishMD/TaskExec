<template>
  <v-form
    ref="form"
    lazy-validation
    v-model="formValid"
    v-if="item != null && secretStorages != null"
  >
    <v-alert :value="!!formError" color="error" class="mb-6">{{ formError }}</v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t('keyName')"
      :rules="[(v) => !!v || $t('name_required')]"
      required
      :disabled="formSaving"
      outlined
      dense
    />

    <v-card
      class="mb-6"
      :color="$vuetify.theme.dark ? '#212121' : 'white'"
      style="background: #8585850f"
    >
      <v-tabs fixed-tabs v-model="sourceStorageTypeIndex" class="KeySourceTabs">
        <v-tab :disabled="formSaving" style="padding: 0"
          >{{ $t('uiLocal') }}</v-tab
        >
        <v-tab
          v-if="supportStorages"
          :disabled="formSaving"
          style="padding: 0"
          >{{ $t('uiStorage') }}</v-tab
        >
        <v-tab :disabled="formSaving" style="padding: 0">
          {{ $t('uiEnv') }}
        </v-tab>
        <v-tab :disabled="formSaving" style="padding: 0">
          {{ $t('uiFile') }}
        </v-tab>
      </v-tabs>

      <div
        :class="!supportStorages && sourceStorageType === 'vault' ? '' : 'ml-4 mr-4 mt-6'"
        v-if="sourceStorageType"
      >

        <v-autocomplete
          v-if="supportStorages && sourceStorageType === 'vault'"
          v-model="item.source_storage_id"
          @change="item.source_mapping = {}"
          :rules="[(v) => !!v || $t('required')]"
          :label="$t('uiStorage')"
          :items="secretStorages"
          item-value="id"
          item-text="name"
          :disabled="formSaving"
          outlined
          dense
          clearable
        />

        <VaultSecretPath
          v-if="supportStorages && sourceStorageType === 'vault' && item.source_storage_id != null"
          v-model="item.source_storage_key" :project-id="projectId"
          :storage-id="item.source_storage_id" :disabled="formSaving"
          @change="item.source_mapping = {}"
        />

        <v-text-field
          v-if="['env', 'file'].includes(sourceStorageType)"
          v-model="item.source_storage_key"
          :label="
            sourceStorageType === 'env' ? $t('uiEnvironmentVariableName') : $t('uiPathToTheFile')
          "
          :rules="[(v) => !!v || $t('type_required')]"
          :disabled="formSaving || !canEditSecrets"
          outlined
          dense
        />
      </div>
    </v-card>

    <v-select
      v-model="item.type"
      :label="$t('type')"
      :rules="[(v) => !!v || !canEditSecrets || $t('type_required')]"
      :items="inventoryTypes"
      item-value="id"
      item-text="name"
      :required="canEditSecrets"
      :disabled="formSaving || !canEditSecrets"
      outlined
      dense
    />

    <VaultKeyMapping v-if="sourceStorageType === 'vault'"
      v-model="item.source_mapping" :project-id="projectId" :storage-id="item.source_storage_id"
      :path="item.source_storage_key" :type="item.type" :disabled="formSaving" />
    <v-textarea v-if="!isExternalReference && item.type === 'object'"
      v-model="objectText" :label="$t('keyVariableSet')" outlined rows="5"
      :disabled="formSaving || !canEditSecrets" :rules="[validateObject]" />

    <v-text-field
      v-if="!isExternalReference && item.type === 'string'"
      v-model="item.string"
      :label="$t('uiSecret')"
      :type="showString ? 'text' : 'password'"
      :append-icon="showString ? 'mdi-eye' : 'mdi-eye-off'"
      @click:append="showString = !showString"
      :rules="[(v) => !canEditSecrets || !!v || $t('required')]"
      :disabled="formSaving || !canEditSecrets"
      autocomplete="new-password"
      outlined dense
    />

    <v-text-field
      v-model="item.login_password.login"
      :label="$t('usernameOptional')"
      v-if="!isExternalReference && item.type === 'login_password'"
      :disabled="formSaving || !canEditSecrets"
      outlined
      dense
    />

    <v-text-field
      v-model="item.login_password.password"
      :append-icon="showLoginPassword ? 'mdi-eye' : 'mdi-eye-off'"
      :label="$t('password')"
      :rules="[(v) => !!v || !canEditSecrets || $t('password_required')]"
      :class="{ 'masked-secret-input': !showLoginPassword }"
      v-if="!isExternalReference && item.type === 'login_password'"
      :required="canEditSecrets"
      :disabled="formSaving || !canEditSecrets"
      autocomplete="new-password"
      @click:append="showLoginPassword = !showLoginPassword"
      outlined
      dense
    />

    <v-text-field
      v-model="item.ssh.login"
      :label="$t('usernameOptional')"
      v-if="!isExternalReference && item.type === 'ssh'"
      :disabled="formSaving || !canEditSecrets"
      outlined
      dense
    />

    <v-text-field
      v-model="item.ssh.passphrase"
      :append-icon="showSSHPassphrase ? 'mdi-eye' : 'mdi-eye-off'"
      :label="$t('uiPassphraseOptional')"
      :class="{ 'masked-secret-input': !showSSHPassphrase }"
      v-if="!isExternalReference && item.type === 'ssh'"
      :disabled="formSaving || !canEditSecrets"
      @click:append="showSSHPassphrase = !showSSHPassphrase"
      outlined
      dense
    />

    <v-checkbox
      v-model="item.generate_ssh_key"
      :label="$t('uiGenerateSSHKey')"
      v-if="!isExternalReference && item.type === 'ssh'"
      :disabled="formSaving || !canEditSecrets"
      class="mt-0 mb-3"
      hide-details
    />

    <v-textarea
      outlined
      v-model="item.ssh.private_key"
      :label="$t('privateKey')"
      :disabled="formSaving || !canEditSecrets || item.generate_ssh_key"
      :rules="
        [
          (v) => !canEditSecrets || item.generate_ssh_key || !!v || $t('private_key_required')
        ]"
      v-if="!isExternalReference && item.type === 'ssh'"
    />

    <div
      v-if="!replaceLocalSecret && !isExternalReference && !sourceChanged
        && item.type === 'ssh' && !isNew && hasGeneratedPublicKey"
      class="mb-4"
    >
      <div class="pb-1">{{ $t('uiPublicKey') }}</div>
      <div style="position: relative">
        <pre
          style="
            overflow: hidden;
            background: gray;
            color: white;
            border-radius: 10px;
            margin-top: 0;
            white-space: normal;
            height: 35px;
          "
          :style="{height: showPublicKey ? 'auto' : '35px' }"
          class="pa-2"
          >{{ publicKey }}</pre
        >

        <v-btn
          style="position: absolute; right: 40px; top: 0; transform: scale(0.9);"
          text
          @click="showPublicKey = !showPublicKey"
        >
          {{ showPublicKey ? $t('hide') : $t('show') }}
        </v-btn>

        <CopyClipboardButton
          style="position: absolute; right: 0; top: 0; transform: scale(0.9);"
          :text="publicKey"
        />
      </div>
    </div>

    <v-checkbox
        v-model="replaceLocalSecret"
        :label="$t('override')"
        v-if="!isNew && !isExternalReference && !sourceChanged"
        hide-details
        class="mt-0"
    />

    <v-alert dense text type="info" v-if="item.type === 'none'">
      {{ $t('useThisTypeOfKeyForHttpsRepositoriesAndForPlaybook') }}
    </v-alert>
  </v-form>
</template>
<script>
import ItemFormBase from '@/components/ItemFormBase';
import VaultSecretPath from '@/components/VaultSecretPath.vue';
import VaultKeyMapping from '@/components/VaultKeyMapping.vue';
import CopyClipboardButton from '@/components/CopyClipboardButton.vue';

export default {
  components: {
    CopyClipboardButton,
    VaultKeyMapping,
    VaultSecretPath,
  },

  mixins: [ItemFormBase],

  props: {
    supportStorages: Boolean,
  },

  data() {
    return {
      showLoginPassword: false,
      showSSHPassphrase: false,
      showString: false,
      objectText: '{}',
      secretStorages: null,
      showPublicKey: false,
      savedSource: null,
      sourceDrafts: {},
      replaceLocalSecret: false,
    };
  },

  computed: {
    hasGeneratedPublicKey() {
      return this.publicKey !== '';
    },

    publicKey: {
      get() {
        try {
          const plain = JSON.parse(this.item?.plain || '{}');
          return plain.public_key || '';
        } catch (e) {
          return '';
        }
      },
    },

    sourceStorageType() {
      return this.item?.source_storage_type;
    },

    sourceStorageTypes() {
      return [undefined, ...(this.supportStorages ? ['vault'] : []), 'env', 'file'];
    },

    sourceStorageTypeIndex: {
      get() {
        return Math.max(0, this.sourceStorageTypes.indexOf(this.item.source_storage_type));
      },
      set(index) {
        const nextSource = this.sourceStorageTypes[index];
        if ((nextSource || null) === (this.sourceStorageType || null)) return;
        this.sourceDrafts[this.sourceStorageType || 'local'] = {
          type: this.item.type,
          source_mapping: this.item.source_mapping,
          objectText: this.objectText,
          source_storage_id: this.item.source_storage_id,
          source_storage_key: this.item.source_storage_key,
          ssh: { ...this.item.ssh },
          login_password: { ...this.item.login_password },
          string: this.item.string,
          generate_ssh_key: this.item.generate_ssh_key,
        };
        const draft = this.sourceDrafts[nextSource || 'local'] || {
          type: nextSource && this.item.type === 'none' ? 'string' : this.item.type,
          source_storage_id: null,
          source_mapping: {},
          objectText: '{}',
          source_storage_key: '',
          ssh: {},
          login_password: {},
          string: '',
          generate_ssh_key: false,
        };
        this.objectText = draft.objectText;
        const { objectText, ...sourceDraft } = draft;
        this.item = {
          ...this.item,
          ...sourceDraft,
          source_storage_type: nextSource,
          override_secret: false,
          reference_only: false,
        };
      },
    },

    canEditSecrets() {
      return this.isNew || this.isExternalReference
        || this.sourceChanged || this.replaceLocalSecret;
    },

    sourceChanged() {
      return (this.sourceStorageType || null) !== this.savedSource;
    },

    isExternalReference() {
      return ['vault', 'env', 'file'].includes(this.sourceStorageType);
    },

    inventoryTypes() {
      return [
        { id: 'string', name: this.$t('uiString') },
        { id: 'object', name: this.$t('keyVariableSet') },
        { id: 'ssh', name: this.$t('keyFormSshKey') },
        { id: 'login_password', name: this.$t('keyFormLoginPassword') },
        ...(!this.isExternalReference ? [{ id: 'none', name: this.$t('keyFormNone') }] : []),
      ];
    },

  },

  async created() {
    [this.secretStorages] = await Promise.all([this.loadProjectResources('secret_storages')]);
  },

  methods: {
    validateObject(value) {
      if (!this.canEditSecrets) return true;
      try {
        const obj = JSON.parse(value);
        return (obj !== null && typeof obj === 'object' && !Array.isArray(obj))
          || this.$t('extraVarsObjectRequired');
      } catch (err) { return this.$t('extraVarsObjectRequired'); }
    },
    afterLoadData() {
      this.$set(this.item, 'source_mapping', this.item.source_mapping || {});
      this.objectText = JSON.stringify(this.item.object || {}, null, 2);
      this.savedSource = this.sourceStorageType || null;
      this.sourceDrafts = {};
      this.replaceLocalSecret = false;
      this.item.ssh = this.item.ssh || {};
      this.item.login_password = this.item.login_password || {};
    },

    beforeSave() {
      this.item.reference_only = this.sourceStorageType === 'vault';
      this.item.override_secret = this.sourceChanged || this.replaceLocalSecret;
      if (this.sourceStorageType !== 'vault') {
        this.item.source_storage_id = null;
        this.item.source_mapping = null;
      }
      this.item.object = !this.isExternalReference && this.item.type === 'object'
        && this.canEditSecrets ? JSON.parse(this.objectText) : null;
      if (!this.isExternalReference) this.item.source_storage_key = null;
      if (this.isExternalReference) {
        this.item.override_secret = this.sourceStorageType !== 'vault';
        this.item.generate_ssh_key = false;
        this.item.ssh = {};
        this.item.login_password = {};
        this.item.string = '';
      }
      // The checkbox is hidden for non-ssh types and read-only storages but
      // keeps its value, and the server rejects generate_ssh_key in both cases.
      // Generation only makes sense when the secret is being overridden.
      if (this.item.type !== 'ssh' || this.isExternalReference || (!this.isNew && !this.item.override_secret)) {
        this.item.generate_ssh_key = false;
      }
    },

    getNewItem() {
      return {
        ssh: {},
        login_password: {},
        generate_ssh_key: false,
      };
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/keys`;
    },

    getSingleItemUrl() {
      return `/api/project/${this.projectId}/keys/${this.itemId}`;
    },
  },
};
</script>

<style lang="scss">
@media (max-width: 599px) {
  .KeySourceTabs {
    .v-tabs-bar { height: auto !important; }
    .v-slide-group__content {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      flex: 1 1 auto;
      transform: none !important;
    }
    .v-tab { min-width: 0; min-height: 44px; }
    .v-tabs-slider-wrapper, .v-slide-group__prev, .v-slide-group__next { display: none; }
  }
}
</style>
