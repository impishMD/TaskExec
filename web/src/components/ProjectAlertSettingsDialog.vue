<template>
  <v-dialog :value="value" max-width="640" persistent content-class="item-dialog"
    @input="$emit('input', $event)">
    <v-card v-if="value" class="project-alert-dialog" data-testid="project-alert-settings-dialog">
      <v-card-title>
        <div class="item-dialog__title-text">{{ $t('alertsConfigure') }}</div>
        <div class="item-dialog__title-actions">
          <v-btn icon :disabled="busy" :aria-label="$t('close')" @click="$emit('input', false)"
            data-testid="alerts-close">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </div>
      </v-card-title>
      <v-tabs :value="0" class="px-4">
        <v-tab><v-icon left small>mdi-send</v-icon>Telegram</v-tab>
      </v-tabs>
      <v-divider />
      <TelegramAlertSettings
        :project-id="projectId"
        @busy="busy = $event"
        @saved="$emit('input', false)"
        @cancel="$emit('input', false)"
      />
    </v-card>
  </v-dialog>
</template>
<script>
import TelegramAlertSettings from '@/components/TelegramAlertSettings.vue';

export default {
  components: { TelegramAlertSettings },
  props: { value: Boolean, projectId: Number },
  data: () => ({ busy: false }),
  watch: { value() { this.busy = false; } },
};
</script>

<style lang="scss">
.project-alert-dialog {
  display: flex;
  flex-direction: column;
  max-height: 90vh;
  overflow: hidden;

  > .v-card__title, > .v-tabs, .v-divider, .v-card__actions {
    flex: 0 0 auto;
  }

  .telegram-alert-settings, .v-form {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
  }

  .telegram-alert-fields {
    min-height: 0;
    overflow-y: auto;
  }
}
</style>
