<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="project-settings-page">
    <YesNoDialog
      v-model="deleteProjectDialog"
      :title="$t('deleteProject')"
      :text="$t('askDeleteProj')"
      @yes="deleteProject()"
    />

    <v-toolbar flat >
      <v-app-bar-nav-icon @click="showDrawer()"></v-app-bar-nav-icon>
      <v-toolbar-title>{{ $t('dashboard') }}</v-toolbar-title>
    </v-toolbar>

    <DashboardMenu
      :project-id="projectId"
      :project-type="projectType"
      :can-update-project="true"
    />

    <v-tabs v-model="settingsTab" show-arrows data-testid="project-settings-tabs">
      <v-tab href="#general" data-testid="settings-tab-general">
        {{ $t('general_settings') }}
      </v-tab>
      <v-tab href="#alerts" data-testid="settings-tab-alerts">
        {{ $t('alertsTitle') }}
      </v-tab>
      <v-tab v-if="canManageTokens" href="#tokens" data-testid="settings-tab-tokens">
        {{ $t('api_tokens') }}
      </v-tab>
      <v-tab href="#danger" data-testid="settings-tab-danger">
        {{ $t('danger_zone_settings') }}
      </v-tab>
    </v-tabs>

    <v-tabs-items v-model="settingsTab" :key="projectId" class="project-settings-panels">
      <v-tab-item value="general" eager>
        <div class="project-settings-form">
          <div>
            <ProjectForm
              :item-id="projectId"
              ref="form"
              @error="onError"
              @save="onSave"
              :system-info="systemInfo"
            />
          </div>

          <div class="d-flex justify-end mt-4">
            <v-btn color="primary" @click="saveProject()" data-testid="settings-saveProject">
              {{ $t('save') }}
            </v-btn>
          </div>
        </div>
      </v-tab-item>

      <v-tab-item value="alerts">
        <v-card outlined data-testid="project-alert-settings">
          <v-tabs v-model="alertChannel" show-arrows class="px-4 pt-2"
            data-testid="project-alert-channels">
            <v-tab href="#telegram" data-testid="alerts-channel-telegram">
              <v-icon left size="20">$vuetify.icons.telegram</v-icon>Telegram
            </v-tab>
          </v-tabs>
          <v-divider />
          <v-tabs-items v-model="alertChannel">
            <v-tab-item value="telegram">
              <TelegramAlertSettings :project-id="projectId" />
            </v-tab-item>
          </v-tabs-items>
        </v-card>
      </v-tab-item>

      <v-tab-item v-if="canManageTokens" value="tokens">
        <ProjectTokens :project-id="projectId" />
      </v-tab-item>

      <v-tab-item value="danger">
        <div class="project-settings-form" data-testid="project-danger-settings">
          <div class="project-backup project-settings-button" v-if="projectType === ''">
            <v-row align="center">
              <v-col cols="12" sm="auto">

                <v-btn
                  color="primary"
                  @click="backupProject"
                  :disabled="backupProgress"
                  min-width="170"
                  data-testid="settings-exportProject"
                >{{ $t('backup') }}
                </v-btn>

                <v-progress-linear
                  v-if="backupProgress"
                  color="primary accent-4"
                  indeterminate
                  rounded
                  height="36"
                  style="margin-top: -36px"
                ></v-progress-linear>

              </v-col>
              <v-col cols="12" sm>
                <div style="font-size: 14px;">
                  {{ $t('downloadTheProjectBackupFile') }}
                </div>
              </v-col>
            </v-row>
          </div>

          <div class="project-backup project-settings-button" v-if="projectType === ''">
            <v-row align="center">
              <v-col cols="12" sm="auto">

                <v-btn
                  color="blue-grey"
                  @click="clearCache"
                  :disabled="clearCacheProgress"
                  min-width="170"
                  data-testid="settings-clearCache"
                >{{ $t('clear_cache') }}</v-btn>

                <v-progress-linear
                  v-if="clearCacheProgress"
                  color="blue-grey darken-1"
                  indeterminate
                  rounded
                  height="36"
                  style="margin-top: -36px"
                ></v-progress-linear>

              </v-col>
              <v-col cols="12" sm>
                <div style="font-size: 14px">
                  {{ $t('clear_cache_message') }}
                </div>
              </v-col>
            </v-row>
          </div>

          <div class="project-delete-form project-settings-button">
            <v-row align="center">
              <v-col cols="12" sm="auto">
                <v-btn
                  color="error"
                  min-width="170"
                  @click="deleteProjectDialog = true"
                  data-testid="settings-deleteProject"
                >{{ $t('deleteProject2') }}
                </v-btn>
              </v-col>
              <v-col cols="12" sm>
                <div style="font-size: 14px; color: #ff5252">
                  {{ $t('onceYouDeleteAProjectThereIsNoGoingBackPleaseBeCer') }}
                </div>
              </v-col>
            </v-row>
          </div>
        </div>
      </v-tab-item>
    </v-tabs-items>
  </div>
</template>
<style lang="scss">
  @import '~vuetify/src/styles/styles.sass';

  .project-settings-page .project-settings-panels {
    margin-top: 20px;
    background: transparent;
    .project-settings-form { margin: 0; }
  }

  .project-settings-button {
    margin: 24px 0;
    .v-btn {
      max-width: 100%;
      height: auto !important;
      min-height: 38px;
      padding-top: 8px;
      padding-bottom: 8px;
      .v-btn__content { white-space: normal; }
    }
    &:first-child { margin-top: 0; }
    &:last-child { margin-bottom: 0; }

    @media #{map-get($display-breakpoints, 'sm-and-down')} {
      padding: 0 6px;
    }
  }
</style>
<script>
import EventBus from '@/event-bus';
import ProjectForm from '@/components/ProjectForm.vue';
import TelegramAlertSettings from '@/components/TelegramAlertSettings.vue';
import ProjectTokens from '@/components/ProjectTokens.vue';
import { getErrorMessage } from '@/lib/error';
import axios from 'axios';
import YesNoDialog from '@/components/YesNoDialog.vue';
import delay from '@/lib/delay';
import DashboardMenu from '@/components/DashboardMenu.vue';

export default {
  components: {
    DashboardMenu, YesNoDialog, ProjectForm, TelegramAlertSettings, ProjectTokens,
  },
  props: {
    projectId: Number,
    projectType: String,
    systemInfo: Object,
    userRole: String,
    isAdmin: Boolean,
  },

  computed: {
    canManageTokens() { return this.isAdmin || this.userRole === 'owner'; },
  },

  data() {
    return {
      settingsTab: 'general',
      alertChannel: 'telegram',
      deleteProjectDialog: null,
      backupProgress: false,
      clearCacheProgress: false,
    };
  },

  watch: {
    projectId() {
      this.settingsTab = 'general';
      this.alertChannel = 'telegram';
    },
  },

  methods: {
    showDrawer() {
      EventBus.$emit('i-show-drawer');
    },

    onError(e) {
      EventBus.$emit('i-snackbar', {
        color: 'error',
        text: e.message,
      });
    },

    onSave(e) {
      EventBus.$emit('i-project', {
        action: 'edit',
        item: e.item,
      });
    },

    async saveProject() {
      await this.$refs.form.save();
    },

    async clearCache() {
      this.clearCacheProgress = true;
      await delay(1000);

      try {
        await axios({
          method: 'delete',
          url: `/api/project/${this.projectId}/cache`,
          transformResponse: (res) => res, // Necessary to not parse json
          responseType: 'json',
        });

        await delay(1000);

        EventBus.$emit('i-snackbar', {
          color: 'success',
          text: this.$t('uiProjectCacheCleaned'),
        });
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      } finally {
        this.clearCacheProgress = false;
      }
    },

    async backupProject() {
      this.backupProgress = true;
      await delay(1000);

      try {
        const backup = await axios({
          method: 'get',
          url: `/api/project/${this.projectId}/backup`,
          transformResponse: (res) => res, // Necessary to not parse json
          responseType: 'json',
        });

        const a = document.createElement('a');
        const blob = new Blob([backup.data], { type: 'application/json' });
        a.download = `backup_${this.projectId}_${Date.now()}.json`;
        a.href = URL.createObjectURL(blob);
        a.click();

        await delay(1000);

        EventBus.$emit('i-snackbar', {
          color: 'success',
          text: this.$t('uiProjectExported'),
        });
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      } finally {
        this.backupProgress = false;
      }
    },

    async deleteProject() {
      try {
        await axios({
          method: 'delete',
          url: `/api/project/${this.projectId}`,
          responseType: 'json',
        });
        EventBus.$emit('i-project', {
          action: 'delete',
          item: {
            id: this.projectId,
          },
        });
      } catch (err) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(err),
        });
      }
    },
  },
};
</script>
