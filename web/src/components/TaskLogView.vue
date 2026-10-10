<template>
  <div class="task-log-view">
    <div v-if="item.message || item.commit_message" class="task-log-view__message">
      <span v-if="item.message"><v-icon small>mdi-message-outline</v-icon> {{ item.message }}</span>
      <span v-if="item.commit_message">
        <v-icon small>mdi-source-fork</v-icon> {{ item.commit_message }}
      </span>
    </div>

    <div class="task-log-view__toolbar">
      <div class="task-log-view__status">
        <TaskStatus :status="item.status" :project-id="projectId" :task-id="itemId"
          :output="output.concat(outputBuffer)" data-testid="task-status" />
        <span class="task-log-view__status_part">
          {{ item.project_token_id
            ? $t('projectTokenStartedBy', {
              name: item.project_token_name, time: $options.filters.formatDate(item.start),
            }) : user
            ? $t('taskStartedByAt', {
              user: user.name, time: $options.filters.formatDate(item.start),
            })
            : $t('taskStartedAt', { time: $options.filters.formatDate(item.start) }) }}
        </span>
        <span class="task-log-view__status_part">
          <v-icon small>mdi-clock-outline</v-icon>
          {{ [item.start, item.end] | formatMilliseconds }}
        </span>
      </div>
      <v-tabs class="task-log-view__tabs" v-model="tab" show-arrows>
        <v-tab>{{ $t('uiLog') }}</v-tab>
        <v-tab>{{ $t('template_details') }}</v-tab>
        <v-tab v-if="summaryAvailable" :disabled="!isTaskStopped || !item.end">
          {{ $t('uiSummary') }}
        </v-tab>
      </v-tabs>
    </div>

    <div v-if="tab === 0" class="task-log-view__log">
      <VirtualList
        class="task-log-records taskexec-scrollbar"
        data-key="id"
        :data-sources="output"
        :data-component="itemComponent"
        :estimate-size="22"
        :keeps="100"
        ref="records"
      />
      <div class="task-log-view__actions" v-if="canStop || isTaskStopped">
        <template v-if="item.status === 'waiting_confirmation'">
          <v-btn color="success" :aria-label="$t('confirmTask')" @click="confirmTask()">
            <v-icon>mdi-check</v-icon>
          </v-btn>
          <v-btn color="warning" :aria-label="$t('status_rejected')" @click="rejectTask()">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </template>
        <v-btn v-if="canStop" color="error" @click="stopTask(item.status === 'stopping')">
          {{ item.status === 'stopping' ? $t('forceStop') : $t('stop') }}
        </v-btn>
        <v-btn v-if="isTaskStopped" color="blue-grey" :href="rawLogURL" target="_blank"
          rel="noopener" data-testid="task-rawLog">{{ $t('raw_log') }}</v-btn>
      </div>
    </div>

    <div v-else-if="tab === 1" class="task-log-view__panel taskexec-scrollbar">
      <v-container fluid class="px-5 py-4">
        <TaskDetails :item="item" :user="user" :schedule="schedule"
          :integration="integration" :project-id="projectId" />
      </v-container>
    </div>
    <div v-else-if="tab === 2" class="task-log-view__panel taskexec-scrollbar">
      <AnsibleStageView :project-id="projectId" :task-id="itemId" />
    </div>
  </div>
</template>

<style lang="scss">
.task-log-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  min-width: 0;
}
.task-log-view__message {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  padding: 0 24px 12px;
  overflow-wrap: anywhere;
  flex: 0 0 auto;
}
.task-log-view__toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
  padding: 0 24px 12px;
  flex: 0 0 auto;
  border-bottom: 1px solid var(--taskexec-border);
}
.task-log-view__status {
  display: flex;
  flex: 1 1 auto;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.task-log-view__status_part {
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--taskexec-soft);
  overflow-wrap: anywhere;
}
.task-log-view__tabs.v-tabs {
  flex: 0 1 auto;
  width: auto;
  max-width: 100%;
  margin-left: auto;
}
.task-log-view__log {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
.task-log-view__actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  flex: 0 0 auto;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--taskexec-border);
}
.task-log-records {
  background: #000;
  color: #fff;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  font-family: monospace;
  margin: 0;
  padding: 12px 16px;
}
.task-log-view__panel {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}
.task-log-records__record { display: flex; align-items: flex-start; }
.task-log-records__time { flex: 0 0 100px; user-select: none; }
.task-log-records__output { flex: 1; min-width: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 600px) {
  .task-log-view__toolbar { padding-left: 16px; padding-right: 16px; }
  .task-log-records__time { flex-basis: 72px; }
  .task-log-view__actions { padding: 12px 16px; }
}
</style>
<script>
import axios from 'axios';
import TaskStatus from '@/components/TaskStatus.vue';
import socket from '@/socket';
import VirtualList from 'vue-virtual-scroll-list';
import TaskLogViewRecord from '@/components/TaskLogViewRecord.vue';
import ProjectMixin from '@/components/ProjectMixin';
import AnsibleStageView from '@/components/AnsibleStageView.vue';
import TaskDetails from '@/components/TaskDetails.vue';

export default {
  components: {
    TaskDetails, AnsibleStageView, TaskStatus, VirtualList,
  },

  mixins: [ProjectMixin],

  props: {
    item: Object,
    projectId: Number,
    systemInfo: Object,
    app: String,
  },

  data() {
    return {
      tab: 0,
      itemComponent: TaskLogViewRecord,
      output: [],
      outputBuffer: [],
      user: {},
      // The schedule or integration that started this task, resolved by id so
      // the details panel can name and link it. Null when the task was started
      // by a person, or when the origin has since been deleted.
      schedule: null,
      integration: null,
      autoScroll: true,
      // stages: null,
    };
  },

  watch: {
    async itemId() {
      this.reset();
      await this.loadData();
    },

    async projectId() {
      this.reset();
      await this.loadData();
    },

    // async tab() {
    //   if (this.tab === 1) {
    //     this.stages = await this.loadProjectEndpoint(`/tasks/${this.itemId}/stages`);
    //   }
    // },
  },

  computed: {
    summaryAvailable() {
      return this.app === 'ansible' && !!this.systemInfo?.features?.task_summary;
    },

    itemId() {
      return this.item?.id;
    },

    isTaskStopped() {
      return [
        'stopped',
        'error',
        'success',
        'canceled',
        'rejected',
      ].includes(this.item.status);
    },

    rawLogURL() {
      return `${this.systemInfo?.web_host || ''}/api/project/${this.projectId}/tasks/${this.itemId}/raw_output`;
    },

    canStop() {
      return [
        'running',
        'stopping',
        'waiting',
        'starting',
        'waiting_confirmation',
        'confirmed',
        'rejected',
      ].includes(this.item.status);
    },

  },

  async created() {
    this.outputInterval = setInterval(() => {
      this.$nextTick(() => {
        const len = this.outputBuffer.length;
        if (len === 0) {
          return;
        }

        const scrollContainer = this.$refs.records?.$el;
        if (!scrollContainer) {
          return;
        }

        // Check if the current position is already at the bottom
        const currentScrollTop = scrollContainer.scrollTop;
        const maxScrollTop = scrollContainer.scrollHeight - scrollContainer.clientHeight;

        // Add a new item to the list
        this.output.push(...this.outputBuffer.splice(0, len));

        // If the user is already at the bottom, keep it scrolled to the bottom
        // Otherwise, maintain the current scroll position
        this.$nextTick(() => {
          if (Math.abs(currentScrollTop - maxScrollTop) <= 1) {
            // User is at the bottom, scroll to the bottom
            scrollContainer.scrollTop = scrollContainer.scrollHeight;
          } else {
            // User is not at the bottom, preserve current scroll position
            scrollContainer.scrollTop = currentScrollTop;
          }
        });
      });
    }, 1000);
    this.socketListenerId = socket.addListener((data) => this.onWebsocketDataReceived(data));
    await this.loadData();
  },

  beforeDestroy() {
    clearInterval(this.outputInterval);
    socket.removeListener(this.socketListenerId);
  },

  methods: {
    async confirmTask() {
      await axios({
        method: 'post',
        url: `/api/project/${this.projectId}/tasks/${this.itemId}/confirm`,
        responseType: 'json',
        data: {},
      });
    },

    async rejectTask() {
      await axios({
        method: 'post',
        url: `/api/project/${this.projectId}/tasks/${this.itemId}/reject`,
        responseType: 'json',
        data: {},
      });
    },

    async stopTask(force) {
      await axios({
        method: 'post',
        url: `/api/project/${this.projectId}/tasks/${this.itemId}/stop`,
        responseType: 'json',
        data: {
          force,
        },
      });
    },

    reset() {
      this.tab = 0;
      this.output = [];
      this.outputBuffer = [];
      this.user = {};
      this.schedule = null;
      this.integration = null;
    },

    onWebsocketDataReceived(data) {
      if (data.project_id !== this.projectId || data.task_id !== this.itemId) {
        return;
      }

      switch (data.type) {
        case 'update':
          Object.assign(this.item, {
            ...data,
            type: undefined,
          });
          break;
        case 'log':
          this.outputBuffer.push({
            ...data,
            id: data.time + data.output,
          });
          break;
        default:
          break;
      }
    },

    // Only a 404 proves the origin is gone. A timeout or a 500 means we simply
    // do not know, and must not be reported to the user as "deleted".
    async loadOrigin(url) {
      try {
        return { status: 'ready', data: (await axios({ method: 'get', url, responseType: 'json' })).data };
      } catch (e) {
        return { status: e.response && e.response.status === 404 ? 'missing' : 'error', data: null };
      }
    },

    async loadData() {
      // Distinguishes "still loading" from "not there", so a valid origin is
      // never briefly labelled as deleted.
      if (this.item.schedule_id) {
        this.schedule = { status: 'loading', data: null };
      }
      if (this.item.integration_id) {
        this.integration = { status: 'loading', data: null };
      }

      // These requests are fired again on every task change; without this the
      // slower response of an earlier task can land after a later one.
      const requested = { project: this.projectId, task: this.itemId };

      const [output, user, schedule, integration] = await Promise.all([

        (await axios({
          method: 'get',
          url: `/api/project/${this.projectId}/tasks/${this.itemId}/output`,
          responseType: 'json',
        })).data.map((item) => ({
          ...item,
          id: item.time + item.output,
        })),

        this.item.user_id ? (await axios({
          method: 'get',
          url: `/api/users/${this.item.user_id}`,
          responseType: 'json',
        })).data : null,

        this.item.schedule_id
          ? this.loadOrigin(`/api/project/${this.projectId}/schedules/${this.item.schedule_id}`)
          : null,

        this.item.integration_id
          ? this.loadOrigin(`/api/project/${this.projectId}/integrations/${this.item.integration_id}`)
          : null,
      ]);

      if (requested.project !== this.projectId || requested.task !== this.itemId) {
        return;
      }

      this.output = output;
      this.user = user;
      this.schedule = schedule;
      this.integration = integration;
    },
  },
};
</script>
