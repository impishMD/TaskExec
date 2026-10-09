<template>
  <div class="telegram-alert-settings">
    <v-progress-linear v-if="loading" indeterminate color="primary" />
    <v-alert v-if="error" type="error" text class="mt-4">{{ error }}</v-alert>
    <v-btn v-if="!settings && !loading" text @click="load">{{ $t('alertsRetry') }}</v-btn>
    <v-btn v-if="projectId && !settings" text class="ma-4" @click="$emit('cancel')">
      {{ $t('cancel') }}
    </v-btn>
    <v-form v-if="settings" ref="form" @submit.prevent="save" :disabled="saving">
      <div class="telegram-alert-fields px-6 pt-5 pb-2">
        <template v-if="projectId">
          <v-switch
            v-model="enabled"
            :label="$t('alertsEnableTelegram')"
            :disabled="saving"
            data-testid="alerts-enable-telegram"
            class="mt-0 mb-4"
            hide-details
            inset
          />
          <p class="text-body-2 text--secondary">{{ $t('alertsProjectHint') }}</p>
          <v-text-field
            v-model="chatId"
            :label="$t('alertsChatId')"
            :hint="$t('alertsChatHint')"
            persistent-hint
            :disabled="!enabled || saving"
            :rules="enabled ? [(v) => !!v.trim() || $t('alertsChatRequired')] : []"
            outlined
            dense
            class="mb-5"
            data-testid="alerts-chat-id"
          />
          <v-checkbox
            v-model="useGlobalToken"
            :label="$t('alertsUseGlobalToken')"
            :disabled="!enabled || saving"
            hide-details
            class="mt-0 mb-3"
            data-testid="alerts-use-global-token"
          />
          <v-alert
            dense text
            :type="settings.global_token_configured ? 'info' : 'warning'"
            class="text-body-2"
          >
            {{ $t(settings.global_token_configured
              ? 'alertsGlobalTokenAvailable' : 'alertsGlobalTokenMissing') }}
          </v-alert>
        </template>
        <p v-else class="text-body-2 text--secondary">{{ $t('alertsGlobalHint') }}</p>
        <v-text-field
          v-model="token"
          type="password"
          autocomplete="new-password"
          :label="$t(projectId ? 'alertsOverrideToken' : 'alertsBotToken')"
          :placeholder="settings.has_token && !clearToken ? $t('alertsTokenSaved') : ''"
          :hint="$t('alertsTokenHint')"
          persistent-hint
          :disabled="saving || (!!projectId && (!enabled || useGlobalToken))"
          :rules="projectId && enabled && !useGlobalToken
            ? [(v) => !!v.trim() || settings.has_token || $t('alertsTokenRequired')] : []"
          outlined
          dense
          data-testid="alerts-bot-token"
        />
        <v-chip
          v-if="settings.has_token && !clearToken"
          small outlined color="success" class="mt-3"
        >
          <v-icon left small>mdi-check-circle-outline</v-icon>
          {{ $t(projectId ? 'alertsOverrideSaved' : 'alertsGlobalSaved') }}
        </v-chip>
        <v-btn
          v-if="!projectId && settings.has_token && !clearToken"
          small text color="error" class="mt-3 ml-2"
          :disabled="saving"
          @click="clearToken = true; token = ''"
          data-testid="alerts-remove-token"
        >{{ $t('alertsRemoveToken') }}</v-btn>
        <p v-if="clearToken" class="text-body-2 warning--text mt-3 mb-0">
          {{ $t('alertsTokenWillBeRemoved') }}
        </p>
      </div>
      <v-divider class="mt-4" />
      <v-card-actions class="px-6 py-4">
        <v-btn v-if="projectId" text :disabled="saving" @click="$emit('cancel')">
          {{ $t('cancel') }}
        </v-btn>
        <v-spacer />
        <v-btn
          color="primary"
          :loading="saving"
          :disabled="!dirty || loading"
          type="submit"
          data-testid="alerts-save"
        >{{ $t('save') }}</v-btn>
      </v-card-actions>
    </v-form>
  </div>
</template>

<script>
import axios from 'axios';
import EventBus from '@/event-bus';
import { getErrorMessage } from '@/lib/error';

export default {
  props: { projectId: { type: Number, default: 0 } },
  data() {
    return {
      loading: false,
      saving: false,
      settings: null,
      enabled: false,
      chatId: '',
      token: '',
      useGlobalToken: true,
      clearToken: false,
      error: '',
    };
  },
  computed: {
    url() {
      return this.projectId
        ? `/api/project/${this.projectId}/alerts/telegram` : '/api/alerts/telegram';
    },
    dirty() {
      return this.settings && (this.token.trim() !== '' || this.clearToken
        || (this.projectId && (this.enabled !== this.settings.enabled
          || this.chatId !== this.settings.chat_id
          || this.useGlobalToken === this.settings.has_token)));
    },
  },
  watch: {
    projectId() { this.load(); },
  },
  created() { this.load(); },
  methods: {
    apply(settings) {
      this.settings = settings;
      this.enabled = settings.enabled;
      this.chatId = settings.chat_id;
      this.useGlobalToken = !settings.has_token;
      this.token = '';
      this.clearToken = false;
    },
    async load() {
      this.loading = true;
      this.error = '';
      this.settings = null;
      this.token = '';
      try {
        this.apply((await axios.get(this.url)).data);
      } catch (err) {
        this.error = getErrorMessage(err);
      } finally {
        this.loading = false;
      }
    },
    async save() {
      if (!this.$refs.form.validate()) return;
      this.error = '';
      this.saving = true;
      const data = { enabled: this.enabled, chat_id: this.chatId };
      if (this.projectId && this.useGlobalToken) data.token = '';
      else if (this.token.trim()) data.token = this.token.trim();
      else if (this.clearToken) data.token = '';
      try {
        this.apply((await axios.put(this.url, data)).data);
        EventBus.$emit('i-snackbar', { color: 'success', text: this.$t('alertsSaved') });
        this.$emit('saved');
      } catch (err) {
        this.error = getErrorMessage(err);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>
