<template>
  <div>
    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer" />
      <v-toolbar-title>{{ $t('alertsTitle') }}</v-toolbar-title>
    </v-toolbar>
    <v-divider />
    <div class="alerts-page mx-auto mt-8">
      <p class="text-body-1 text--secondary mb-6">{{ $t('alertsPageHint') }}</p>
      <v-card outlined>
        <v-tabs :value="0" class="px-4 pt-2">
          <v-tab><v-icon left small>mdi-send</v-icon>Telegram</v-tab>
        </v-tabs>
        <v-divider />
        <TelegramAlertSettings />
      </v-card>
    </div>
  </div>
</template>
<script>
import EventBus from '@/event-bus';
import TelegramAlertSettings from '@/components/TelegramAlertSettings.vue';

export default {
  components: { TelegramAlertSettings },
  methods: { showDrawer() { EventBus.$emit('i-show-drawer'); } },
};
</script>
<style scoped>
.alerts-page { padding-bottom: 32px; }
</style>
