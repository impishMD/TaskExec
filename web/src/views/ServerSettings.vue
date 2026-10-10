<template>
  <div>
    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer" />
      <v-toolbar-title>{{ $t('generalSettings') }}</v-toolbar-title>
    </v-toolbar>
    <v-divider />
    <div class="server-settings-page mx-auto mt-8">
      <p class="text-body-1 text--secondary mb-6">{{ $t('serverSettingsHint') }}</p>
      <v-progress-linear v-if="loading" indeterminate color="primary" />
      <v-alert v-if="error" type="error" text data-testid="server-settings-error">
        {{ error }}
      </v-alert>
      <v-btn v-if="!settings && !loading" text @click="load" data-testid="server-settings-retry">
        {{ $t('alertsRetry') }}
      </v-btn>
      <v-card v-if="settings" outlined class="pa-6">
        <v-form @submit.prevent="save">
          <v-switch
            v-model="useRemoteRunner"
            :label="$t('remoteRunnersDefault')"
            :disabled="saving"
            aria-describedby="remote-runners-hint"
            data-testid="server-remote-runners"
            class="mt-0 mb-4"
            hide-details
            inset
          />
          <p id="remote-runners-hint" class="text-body-2 text--secondary">
            {{ $t('remoteRunnersDefaultHint') }}
          </p>
          <v-btn text small to="/runners" class="mb-4">
            <v-icon left small>mdi-cogs</v-icon>{{ $t('runners') }}
          </v-btn>
          <div class="d-flex justify-end">
            <v-btn
              type="submit" color="primary" :disabled="!dirty || saving" :loading="saving"
              data-testid="server-settings-save"
            >{{ $t('save') }}</v-btn>
          </div>
        </v-form>
      </v-card>
    </div>
  </div>
</template>

<script>
import axios from 'axios';
import EventBus from '@/event-bus';
import { getErrorMessage } from '@/lib/error';

export default {
  data: () => ({
    settings: null,
    useRemoteRunner: false,
    loading: true,
    saving: false,
    error: '',
  }),
  computed: {
    dirty() {
      return this.settings && this.useRemoteRunner !== this.settings.use_remote_runner;
    },
  },
  created() { this.load(); },
  methods: {
    showDrawer() { EventBus.$emit('i-show-drawer'); },
    apply(settings) {
      this.settings = settings;
      this.useRemoteRunner = settings.use_remote_runner;
      EventBus.$emit('i-server-settings', settings);
    },
    async load() {
      this.loading = true;
      this.error = '';
      this.settings = null;
      try {
        this.apply((await axios.get('/api/settings')).data);
      } catch (err) {
        this.error = getErrorMessage(err);
      } finally {
        this.loading = false;
      }
    },
    async save() {
      if (!this.dirty || this.saving) return;
      this.error = '';
      this.saving = true;
      try {
        this.apply((await axios.put('/api/settings', {
          use_remote_runner: this.useRemoteRunner,
        })).data);
        EventBus.$emit('i-snackbar', { color: 'success', text: this.$t('serverSettingsSaved') });
      } catch (err) {
        this.error = getErrorMessage(err);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.server-settings-page { padding-bottom: 32px; }
</style>
