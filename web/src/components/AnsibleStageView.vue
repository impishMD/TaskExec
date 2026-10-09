<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="AnsibleSummary pb-5">
    <v-progress-linear v-if="loading" indeterminate class="mb-4" />
    <v-alert v-if="loadFailed" type="error" text class="ma-5">
      {{ $t('summaryLoadFailed') }}
      <v-btn text small @click="loadData()">{{ $t('alertsRetry') }}</v-btn>
    </v-alert>
    <v-alert v-else-if="!loading && hosts.length === 0" type="info" text class="ma-5">
      {{ $t('summaryNoRecap') }}
    </v-alert>

    <div v-if="hosts.length" class="pa-5 d-flex flex-wrap" style="gap: 10px">
      <div class="AnsibleServerStatus AnsibleServerStatus--ok">
        <div class="AnsibleServerStatus__count">{{ okServers }}</div>
        <div class="AnsibleServerStatus__title">{{ $t('uiOKSERVERS') }}</div>
      </div>

      <div class="AnsibleServerStatus AnsibleServerStatus--bad">
        <div class="AnsibleServerStatus__count">{{ notOkServers }}</div>
        <div class="AnsibleServerStatus__title">{{ $t('uiNOTOKSERVERS') }}</div>
      </div>
    </div>

    <v-btn-toggle class="pl-5 mt-8 mb-3" dense v-model="tab" mandatory>
      <v-btn value="notOkServers">{{ $t('summaryErrors') }}</v-btn>
      <v-btn value="allServers">{{ $t('uiAllServers') }}</v-btn>
    </v-btn-toggle>

    <v-data-table
      v-if="tab === 'notOkServers'"
      hide-default-footer
      single-expand
      show-expand
      :headers="notOkServersHeaders"
      :items="failedTasks"
      :items-per-page="Number.MAX_VALUE"
      class="w-100"
    >
      <template v-slot:item.error="{ item }">
        <div style="overflow: hidden; color: #ff5252; max-width: 400px; text-overflow: ellipsis">
          {{ item.error }}
        </div>
      </template>
      <template v-slot:expanded-item="{ headers, item }">
        <td :colspan="headers.length">
          <pre class="AnsibleSummary__error pa-3">{{ item.error }}</pre>
        </td>
      </template>
    </v-data-table>

    <v-simple-table v-else-if="tab === 'allServers'" class="AnsibleSummary__hosts">
      <template v-slot:default>
        <thead>
          <tr>
            <th>{{ $t('hostConfigTypeHost') }}</th>
            <th>{{ $t('uiChanged') }}</th>
            <th>{{ $t('status_failed') }}</th>
            <th>{{ $t('uiIgnored') }}</th>
            <th>{{ $t('uiOk') }}</th>
            <th>{{ $t('uiRescued') }}</th>
            <th>{{ $t('uiSkipped') }}</th>
            <th>{{ $t('uiUnreachable') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(host, index) in hosts" :key="index">
            <td>{{ host.host }}</td>

            <td
              :style="{
                color: host.changed > 0 ? 'rgb(170,85,0)' : undefined,
                'font-weight': host.changed > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.changed }}
            </td>

            <td
              :style="{
                color: host.failed > 0 ? 'red' : undefined,
                'font-weight': host.failed > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.failed }}
            </td>

            <td
              :style="{
                color: host.ignored > 0 ? 'red' : undefined,
                'font-weight': host.ignored > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.ignored }}
            </td>

            <td
              :style="{
                color: host.ok > 0 ? 'green' : undefined,
                'font-weight': host.ok > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.ok }}
            </td>

            <td
              :style="{
                'font-weight': host.rescued > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.rescued }}
            </td>

            <td
              :style="{
                color: host.skipped > 0 ? 'rgb(0,170,170)' : undefined,
                'font-weight': host.skipped > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.skipped }}
            </td>

            <td
              :style="{
                color: host.unreachable > 0 ? 'red' : undefined,
                'font-weight': host.unreachable > 0 ? 'bold' : undefined,
              }"
            >
              {{ host.unreachable }}
            </td>
          </tr>
        </tbody>
      </template>
    </v-simple-table>
  </div>
</template>
<style lang="scss">
.AnsibleSummary {
  overflow: hidden;
}

.AnsibleSummary__hosts {
  table {
    width: 100%;
  }

  th {
    white-space: normal !important;
    text-transform: none !important;
    letter-spacing: normal !important;
    font-size: 12px !important;
    line-height: 1.4;
    padding: 12px 10px !important;
  }

  td {
    padding: 10px !important;
  }
}

.AnsibleSummary__error {
  overflow: auto;
  background: var(--highlighted-card-bg-color);
  border-radius: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin: 8px 0;
}

.AnsibleServerStatus {
  text-align: center;
  width: 240px;
  max-width: 100%;
  padding: 18px 12px;
  font-weight: 600;
  font-size: 18px;
  line-height: 1.3;
  border-radius: 16px;
}

.AnsibleServerStatus__count {
  font-size: 56px;
  line-height: 1;
  margin-bottom: 8px;
}

.AnsibleServerStatus--ok {
  background: rgba(32, 140, 105, 0.12);
  color: var(--v-primary-base);
  border: 1px solid rgba(32, 140, 105, 0.2);
}

.AnsibleServerStatus--bad {
  background: rgba(239, 71, 111, 0.1);
  color: var(--v-error-base);
  border: 1px solid rgba(239, 71, 111, 0.2);
}
</style>

<script>
import ProjectMixin from '@/components/ProjectMixin';

export default {
  props: {
    projectId: Number,
    taskId: Number,
  },

  mixins: [ProjectMixin],

  data() {
    return {
      loading: false,
      loadFailed: false,
      tab: 'notOkServers',
      failedTasks: [],
      hosts: [],
      loadSequence: 0,
    };
  },

  computed: {
    okServers() {
      return this.hosts.filter((host) => !host.failed && !host.unreachable).length;
    },
    notOkServers() {
      return this.hosts.length - this.okServers;
    },
    notOkServersHeaders() {
      return [
        { text: this.$t('uiServer'), value: 'host', sortable: false },
        { text: this.$t('task2'), value: 'task', sortable: false },
        { text: this.$t('error'), value: 'error', sortable: false },
      ];
    },
  },

  watch: {
    taskId: 'loadData',
    projectId: 'loadData',
  },

  created() {
    this.loadData();
  },

  beforeDestroy() {
    this.loadSequence += 1;
  },

  methods: {
    async loadData() {
      this.loadSequence += 1;
      const sequence = this.loadSequence;
      const { projectId, taskId } = this;
      this.loading = true;
      this.loadFailed = false;
      this.hosts = [];
      this.failedTasks = [];
      try {
        const [failures, hosts] = await Promise.all([
          this.loadEndpoint(`/api/project/${projectId}/tasks/${taskId}/ansible/errors`),
          this.loadEndpoint(`/api/project/${projectId}/tasks/${taskId}/ansible/hosts`),
        ]);
        if (sequence !== this.loadSequence) {
          return;
        }
        this.failedTasks = failures || [];
        this.hosts = hosts || [];
      } catch (e) {
        if (sequence === this.loadSequence) {
          this.loadFailed = true;
        }
      } finally {
        if (sequence === this.loadSequence) {
          this.loading = false;
        }
      }
    },
  },
};
</script>
