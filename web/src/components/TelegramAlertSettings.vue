<template>
  <div class="telegram-alert-settings">
    <v-progress-linear v-if="loading" indeterminate color="primary" />
    <v-alert v-if="error" type="error" text class="mt-4">{{ error }}</v-alert>
    <v-btn v-if="!settings && !loading" text @click="load">{{ $t('alertsRetry') }}</v-btn>
    <div v-if="projectId && !settings" class="d-flex justify-end pa-4">
      <v-btn text color="primary" @click="$emit('cancel')">{{ $t('cancel') }}</v-btn>
    </div>
    <v-form v-if="settings" ref="form" @submit.prevent="save" :disabled="busy">
      <div class="telegram-alert-fields px-6 pt-5 pb-2">
        <template v-if="projectId">
          <v-switch
            v-model="enabled"
            :label="$t('alertsEnableTelegram')"
            :disabled="busy"
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
            :disabled="!enabled || busy"
            :rules="enabled ? [(v) => !!v.trim() || $t('alertsChatRequired')] : []"
            outlined
            dense
            class="mb-5"
            data-testid="alerts-chat-id"
          />
          <v-checkbox
            v-model="useGlobalToken"
            :label="$t('alertsUseGlobalToken')"
            :disabled="!enabled || busy"
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
          :disabled="busy || (!!projectId && (!enabled || useGlobalToken))"
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
          :disabled="busy"
          @click="clearToken = true; token = ''"
          data-testid="alerts-remove-token"
        >{{ $t('alertsRemoveToken') }}</v-btn>
        <p v-if="clearToken" class="text-body-2 warning--text mt-3 mb-0">
          {{ $t('alertsTokenWillBeRemoved') }}
        </p>
        <v-text-field v-if="!projectId" v-model="testChatId" :label="$t('alertsTestChat')"
          :disabled="busy" outlined dense class="mt-5" data-testid="alerts-test-chat" />
      </div>
      <div class="px-6" v-if="testResult">
        <v-alert :type="testResult.ok ? 'success' : 'error'" text class="mb-0"
          data-testid="alerts-test-result">{{ testResult.text }}</v-alert>
      </div>
      <v-divider class="mt-4" />
      <v-card-actions class="px-6 py-4 telegram-alert-actions">
        <v-btn outlined color="primary" type="button" @click="testNotification"
          :loading="testing" :disabled="saving || loading || (!!projectId && !enabled)"
          class="telegram-alert-test"
          data-testid="alerts-test">
          <v-icon left small>mdi-send-check-outline</v-icon>{{ $t('alertsTest') }}
        </v-btn>
        <div class="telegram-alert-save-actions">
          <v-btn v-if="projectId" text color="primary" :disabled="busy" @click="$emit('cancel')"
            data-testid="alerts-cancel">
            {{ $t('cancel') }}
          </v-btn>
          <v-btn
            color="primary"
            :loading="saving"
            :disabled="!dirty || loading || testing"
            type="submit"
            data-testid="alerts-save"
          >{{ $t('save') }}</v-btn>
        </div>
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
      testing: false,
      testChatId: '',
      testResult: null,
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
    busy() { return this.saving || this.testing; },
    testInput() {
      return JSON.stringify([this.chatId, this.testChatId, this.token, this.useGlobalToken,
        this.enabled, this.clearToken]);
    },
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
    busy(value) { this.$emit('busy', value); },
    testInput() { this.testResult = null; },
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
    requestData() {
      const data = { enabled: this.enabled, chat_id: this.chatId };
      if (this.projectId && this.useGlobalToken) data.token = '';
      else if (this.token.trim()) data.token = this.token.trim();
      else if (this.clearToken) data.token = '';
      return data;
    },
    async testNotification() {
      if (this.busy || !this.$refs.form.validate()) return;
      const data = this.requestData();
      data.chat_id = (this.projectId ? this.chatId : this.testChatId).trim();
      if (!data.chat_id) {
        this.testResult = { ok: false, text: this.$t('alertsChatRequired') };
        return;
      }
      data.locale = this.$i18n?.locale || 'en';
      const url = this.url;
      this.testing = true;
      this.testResult = null;
      try {
        await axios.post(`${url}/test`, data);
        if (url === this.url) this.testResult = { ok: true, text: this.$t('alertsTestSent') };
      } catch (err) {
        if (url === this.url) this.testResult = { ok: false, text: getErrorMessage(err) };
      } finally {
        this.testing = false;
      }
    },
    async save() {
      if (!this.$refs.form.validate()) return;
      this.error = '';
      this.saving = true;
      const data = this.requestData();
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

<style lang="scss">
.telegram-alert-actions {
  flex-wrap: wrap;
  gap: 12px;
  .v-btn { margin: 0 !important; }
  .telegram-alert-test {
    max-width: 100%;
    height: auto !important;
    min-height: 38px;
    padding-top: 8px;
    padding-bottom: 8px;
    .v-btn__content { white-space: normal; }
  }
}
.telegram-alert-save-actions {
  display: flex;
  flex-shrink: 0;
  gap: 12px;
  margin-left: auto;
}
</style>
