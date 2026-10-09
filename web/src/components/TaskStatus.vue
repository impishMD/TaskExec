<template>
  <v-chip v-if="status" class="taskexec-task-status" small outlined :color="getStatusColor(status)">
    <v-icon v-if="status !== 'running'" left small>{{ getStatusIcon(status) }}</v-icon>
    <IndeterminateProgressCircular v-else style="margin-left: -5px;" />
    {{ humanizeStatus(status) }}
  </v-chip>
</template>
<script>
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
  },

  methods: {
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
