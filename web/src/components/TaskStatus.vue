<template>
  <v-menu v-model="errorOpen" :disabled="!canShowError" offset-y bottom
    :max-width="640" :close-on-content-click="true" content-class="task-error-popover">
    <template v-slot:activator="{ on, attrs }">
      <v-chip v-if="status" class="taskexec-task-status" small outlined
        :color="getStatusColor(status)" :link="canShowError"
        v-bind="attrs" v-on="canShowError ? on : {}" @click.stop
        :aria-label="canShowError ? $t('taskErrorDetails') : humanizeStatus(status)">
        <v-icon v-if="status !== 'running'" left small>{{ getStatusIcon(status) }}</v-icon>
        <IndeterminateProgressCircular v-else style="margin-left: -5px;" />
        {{ humanizeStatus(status) }}
      </v-chip>
    </template>
    <v-card role="dialog" :aria-label="$t('taskErrorDetails')" data-testid="task-error-popup">
      <v-card-title class="text-subtitle-1">{{ $t('taskErrorDetails') }}</v-card-title>
      <v-progress-linear v-if="errorLoading" indeterminate />
      <v-card-text class="taskexec-scrollbar">
        <div v-if="errorLoading">{{ $t('taskErrorLoading') }}</div>
        <div v-else-if="loadError" class="error--text">{{ loadError }}</div>
        <template v-else>
          <p v-if="!diagnostic.matched">{{ $t('taskErrorMissing') }}</p>
          <pre v-if="diagnostic.text">{{ diagnostic.text }}</pre>
        </template>
      </v-card-text>
    </v-card>
  </v-menu>
</template>
<script>
import axios from 'axios';
import taskErrors from '@/lib/taskErrors';
import { getErrorMessage } from '@/lib/error';
import IndeterminateProgressCircular from '@/components/IndeterminateProgressCircular.vue';

const TaskStatus = Object.freeze({
  WAITING: 'waiting',
  STARTING: 'starting',
  WAITING_CONFIRMATION: 'waiting_confirmation',
  CONFIRMED: 'confirmed',
  REJECTED: 'rejected',
  RUNNING: 'running',
  SUCCESS: 'success',
  ERROR: 'error',
  STOPPING: 'stopping',
  STOPPED: 'stopped',
});

export default {
  components: { IndeterminateProgressCircular },
  props: {
    status: String,
    projectId: Number,
    taskId: Number,
    output: { type: Array, default: null },
  },

  data() {
    return {
      errorOpen: false, errorLoading: false, loadError: '', records: [], requestId: 0,
    };
  },
  computed: {
    canShowError() { return this.status === 'error' && !!this.projectId && !!this.taskId; },
    diagnostic() { return taskErrors(this.output?.length ? this.output : this.records); },
  },
  watch: {
    errorOpen(value) { if (value) this.loadTaskError(); },
    taskId() { this.resetError(); },
    projectId() { this.resetError(); },
    status() { if (!this.canShowError) this.resetError(); },
  },
  beforeDestroy() { this.requestId += 1; },
  methods: {
    resetError() {
      this.errorOpen = false;
      this.errorLoading = false;
      this.loadError = '';
      this.records = [];
      this.requestId += 1;
    },
    async loadTaskError() {
      this.loadError = '';
      this.records = [];
      this.requestId += 1;
      const requestId = this.requestId;
      this.errorLoading = false;
      if (this.output?.length) return;
      this.errorLoading = true;
      try {
        const response = await axios.get(`/api/project/${this.projectId}/tasks/${this.taskId}/output`);
        if (requestId === this.requestId) this.records = response.data;
      } catch (err) {
        if (requestId === this.requestId) this.loadError = getErrorMessage(err);
      } finally {
        if (requestId === this.requestId) this.errorLoading = false;
      }
    },
    getStatusIcon(status) {
      switch (status) {
        case TaskStatus.WAITING:
          return 'mdi-alarm';
        case TaskStatus.STARTING:
          return 'mdi-play-circle';
        case TaskStatus.RUNNING:
          return '';
        case TaskStatus.SUCCESS:
          return 'mdi-check-circle';
        case TaskStatus.ERROR:
          return 'mdi-information';
        case TaskStatus.STOPPING:
          return 'mdi-stop-circle';
        case TaskStatus.STOPPED:
          return 'mdi-stop-circle';
        case TaskStatus.CONFIRMED:
          return 'mdi-check-circle';
        case TaskStatus.WAITING_CONFIRMATION:
          return 'mdi-pause-circle';
        case TaskStatus.REJECTED:
          return 'mdi-close-circle';
        default:
          throw new Error(`Unknown task status ${status}`);
      }
    },

    humanizeStatus(status) {
      switch (status) {
        case TaskStatus.WAITING:
          return this.$t('status_waiting');
        case TaskStatus.STARTING:
          return this.$t('status_starting');
        case TaskStatus.RUNNING:
          return this.$t('running');
        case TaskStatus.SUCCESS:
          return this.$t('status_success');
        case TaskStatus.ERROR:
          return this.$t('status_failed');
        case TaskStatus.STOPPING:
          return this.$t('status_stopping');
        case TaskStatus.STOPPED:
          return this.$t('status_stopped');
        case TaskStatus.CONFIRMED:
          return this.$t('status_confirmed');
        case TaskStatus.WAITING_CONFIRMATION:
          return this.$t('status_waiting_confirmation');
        case TaskStatus.REJECTED:
          return this.$t('status_rejected');
        default:
          throw new Error(`Unknown task status ${status}`);
      }
    },

    getStatusColor(status) {
      switch (status) {
        case TaskStatus.WAITING:
          return '';
        case TaskStatus.STARTING:
          return 'warning';
        case TaskStatus.RUNNING:
          return 'primary';
        case TaskStatus.SUCCESS:
          return 'success';
        case TaskStatus.ERROR:
          return 'error';
        case TaskStatus.STOPPING:
          return '';
        case TaskStatus.STOPPED:
          return '';
        case TaskStatus.CONFIRMED:
          return 'warning';
        case TaskStatus.WAITING_CONFIRMATION:
          return 'warning';
        case TaskStatus.REJECTED:
          return 'error';
        default:
          throw new Error(`Unknown task status ${status}`);
      }
    },
  },
};
</script>

<style lang="scss">
.task-error-popover {
  width: min(640px, calc(100vw - 24px));
  max-width: calc(100vw - 24px) !important;
  .v-card { background: var(--taskexec-surface) !important; }
  .v-card__text { max-height: 50dvh; overflow: auto; }
  pre { white-space: pre-wrap; overflow-wrap: anywhere; color: var(--taskexec-text); }
}
</style>
