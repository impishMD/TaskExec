<template>
  <v-form
    ref="form"
    lazy-validation
    v-model="formValid"
    v-if="templates && item != null"
  >
    <v-alert
      v-model="showInfo"
      color="info"
      text
      dismissible
      class="mb-6"
    >
      <i18n path="scheduleTimezoneHint" tag="span">
        <code place="environmentVariable">TASKEXEC_SCHEDULE_TIMEZONE</code>
        <code place="configParameter">schedule.timezone</code>
      </i18n>
    </v-alert>

    <v-alert
      :value="formError"
      color="error"
      class="pb-2"
    >{{ formError }}
    </v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t('name')"
      :rules="[v => !!v || $t('name_required')]"
      required
      :disabled="formSaving"
      outlined
      dense
    ></v-text-field>

    <v-autocomplete
      v-model="item.template_id"
      :label="$t('template')"
      :items="templates"
      item-value="id"
      :item-text="(itm) => itm.name"
      :rules="[v => !!v || $t('workflowTemplateRequired')]"
      required
      :disabled="formSaving"
      outlined
      dense
    />

    <v-card
      style="background: var(--highlighted-card-bg-color)"
      v-if="item.template_id"
      class="mb-8 pt-3"
    >
      <div style="
        position: absolute;
        background: var(--highlighted-card-bg-color);
        width: 28px;
        height: 28px;
        transform: rotate(45deg);
        left: calc(50% - 14px);
        top: -14px;
        border-radius: 0;
      "></div>

      <v-card-text>
        <TaskParamsForm
          :template="templates.find(t => t.id === item.template_id)"
          v-model="item.task_params"
        />

      </v-card-text>
    </v-card>

    <div v-if="type === 'run_at'">
      <v-text-field

        v-model="runAtInput"
        type="datetime-local"
        :label="$t('uiRunAt')"
        :rules="runAtRules"
        :disabled="formSaving"
        :suffix="$t('timeInZone', { zone: timezone })"
        outlined
        dense
      ></v-text-field>

      <div class="d-flex justify-end">

        <v-checkbox
          v-model="item.delete_after_run"
          hide-details
          class="mt-0 pt-0"
        >
          <template v-slot:label>
            {{ $t('uiDeleteAfterRun') }}
          </template>
        </v-checkbox>
      </div>
    </div>

    <div v-else>
      <v-switch
        v-model="rawCron"
        :label="$t('uiShowCronFormat')"
        :disabled="disableRawCron"
      />

      <v-text-field
        v-if="rawCron"
        v-model="item.cron_format"
        :label="$t('cron')"
        :rules="[v => !!v || $t('uiCronRequired')]"
        required
        :disabled="formSaving"
        @input="refreshCheckboxes()"
        :suffix="$t('timeInZone', { zone: timezone })"
        outlined
        :error="cronFormatError != null"
        :error-messages="cronFormatError"
        dense
      ></v-text-field>

      <div v-else>
        <v-select
          v-model="timing"
          :label="$t('uiTiming')"
          :items="TIMINGS"
          item-value="id"
          item-text="title"
          :rules="[v => !!v || $t('workflowTemplateRequired')]"
          required
          :disabled="formSaving"
          @change="refreshCron()"
          outlined
          hide-details
          dense
        />

        <div v-if="['yearly'].includes(timing)">
          <div class="mt-4">{{ $t('uiMonths') }}</div>
          <div class="d-flex flex-wrap">
            <v-checkbox
              class="mr-2 mt-0 ScheduleCheckbox"
              v-for="m in MONTHS"
              :key="m.id"
              :value="m.id"
              :label="m.title"
              v-model="months"
              color="white"
              :class="{'ScheduleCheckbox--active': months.includes(m.id)}"
              @change="refreshCron()"
            ></v-checkbox>
          </div>
        </div>

        <div v-if="['weekly'].includes(timing)">
          <div class="mt-4">{{ $t('uiWeekdays') }}</div>
          <div class="d-flex flex-wrap">
            <v-checkbox
              class="mr-2 mt-0 ScheduleCheckbox"
              v-for="d in WEEKDAYS" :key="d.id"
              :value="d.id"
              :label="d.title"
              v-model="weekdays"
              color="white"
              :class="{'ScheduleCheckbox--active': weekdays.includes(d.id)}"
              @change="refreshCron()"
            ></v-checkbox>
          </div>
        </div>

        <div v-if="['yearly', 'monthly'].includes(timing)">
          <div class="mt-4">{{ $t('uiDays') }}</div>
          <div class="d-flex flex-wrap">
            <v-checkbox
              class="mr-2 mt-0 ScheduleCheckbox"
              v-for="d in 31"
              :key="d"
              :value="d"
              :label="`${d}`"
              v-model="days"
              color="white"
              :class="{'ScheduleCheckbox--active': days.includes(d)}"
              @change="refreshCron()"
            ></v-checkbox>
          </div>
        </div>

        <div v-if="['yearly', 'monthly', 'weekly', 'daily'].includes(timing)">
          <div class="mt-4 d-flex justify-space-between">
            <span>{{ $t('uiHours') }}</span>
            <b style="color: red;">{{ $t('timeInZone', { zone: timezone }) }}</b>
          </div>
          <div class="d-flex flex-wrap">
            <v-checkbox
              class="mr-2 mt-0 ScheduleCheckbox"
              v-for="h in 24"
              :key="h - 1"
              :value="h - 1"
              :label="`${h - 1}`"
              v-model="hours"
              color="white"
              :class="{'ScheduleCheckbox--active': hours.includes(h - 1)}"
              @change="refreshCron()"
            ></v-checkbox>
          </div>
        </div>

        <div>
          <div class="mt-4">{{ $t('uiMinutes') }}</div>
          <div class="d-flex flex-wrap">
            <v-checkbox
              class="mr-2 mt-0 ScheduleCheckbox"
              v-for="m in MINUTES"
              :key="m.id"
              :value="m.id"
              :label="m.title"
              v-model="minutes"
              color="white"
              :class="{'ScheduleCheckbox--active': minutes.includes(m.id)}"
              @change="refreshCron()"
            ></v-checkbox>
          </div>
        </div>
      </div>
    </div>

    <div
      class="text-center text-subtitle-1 mb-3"
      :class="{'mt-8': !rawCron, 'mt-3': rawCron}"
      style="color: limegreen; font-weight: bold;"
    >{{ $t('uiNextRunTime') }}</div>

    <v-simple-table class="TaskDetails__table text-sub mb-2">
      <template v-slot:default>
        <thead>
        <tr>
          <th>{{ $t('uiTimeZone') }}</th>
          <th>{{ $t('uiDate') }}</th>
          <th>{{ $t('time') }}</th>
        </tr>
        </thead>
        <tbody>
        <tr>
          <td>{{ timezone }}</td>
          <td>{{ nextRunUtcDate }}</td>
          <td>{{ nextRunUtcTime }}</td>
        </tr>
        <tr>
          <td>{{ localTimezone }}</td>
          <td>{{ nextRunLocalDate }}</td>
          <td>{{ nextRunLocalTime }}</td>
        </tr>
        </tbody>
      </template>
    </v-simple-table>

    <v-checkbox
      style="position: absolute; bottom: 15px; left: 22px;"
      v-model="item.active"
      hide-details
    >
      <template v-slot:label>
        {{ $t('enabled') }}
      </template>
    </v-checkbox>

  </v-form>
</template>

<style lang="scss">
.ScheduleCheckbox {

  .v-input__slot {
    padding: 4px 6px;
    font-weight: bold;
    border-radius: 6px;
  }

  .v-messages {
    display: none;
  }

  &.theme--light {
    .v-input__slot {
      background: #e4e4e4;
    }
  }

  &.theme--dark {
    .v-input__slot {
      background: gray;
    }
  }
}

.ScheduleCheckbox--active {
  .v-input__slot {
    background: #4caf50 !important;
  }

  .v-label {
    color: white;
  }
}

</style>

<script>
import ItemFormBase from '@/components/ItemFormBase';
import axios from 'axios';
import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import timezonePlugin from 'dayjs/plugin/timezone';
import customParseFormat from 'dayjs/plugin/customParseFormat';

import { CronExpressionParser } from 'cron-parser';
import {
  isWeekly,
  isYearly,
  isMonthly,
  isDaily,
  isHourly,
  pruneSelectionsForTiming,
  buildCronFormat,
} from '@/lib/cronPresets';
import { getErrorMessage } from '@/lib/error';
import TaskParamsForm from '@/components/TaskParamsForm.vue';

dayjs.extend(utc);
dayjs.extend(timezonePlugin);
dayjs.extend(customParseFormat);

const TIMINGS = [{
  id: 'yearly',
  title: 'timingYearly',
}, {
  id: 'monthly',
  title: 'timingMonthly',
}, {
  id: 'weekly',
  title: 'timingWeekly',
}, {
  id: 'daily',
  title: 'timingDaily',
}, {
  id: 'hourly',
  title: 'timingHourly',
}];

const MINUTES = [
  { id: 0, title: ':00' },
  { id: 5, title: ':05' },
  { id: 10, title: ':10' },
  { id: 15, title: ':15' },
  { id: 20, title: ':20' },
  { id: 25, title: ':25' },
  { id: 30, title: ':30' },
  { id: 35, title: ':35' },
  { id: 40, title: ':40' },
  { id: 45, title: ':45' },
  { id: 50, title: ':50' },
  { id: 55, title: ':55' },
];

const RUN_AT_FORMAT = 'YYYY-MM-DDTHH:mm';

function formatDateInTZ(date, tz) {
  if (date == null) {
    return '—';
  }
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date);

  const get = (type) => parts.find((p) => p.type === type)?.value;

  return `${get('year')}-${get('month')}-${get('day')}`;
}

function formatTimeInTZ(date, tz) {
  if (date == null) {
    return '—';
  }

  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date);

  const get = (type) => parts.find((p) => p.type === type)?.value;

  return `${get('hour')}:${get('minute')}`;
}

export default {
  components: { TaskParamsForm },
  mixins: [ItemFormBase],

  data() {
    return {
      templates: null,
      timing: 'hourly',
      TIMINGS: TIMINGS.map((item) => ({ ...item, title: this.$t(item.title) })),
      MINUTES,
      minutes: [],
      hours: [],
      days: [],
      months: [],
      weekdays: [],
      rawCron: false,
      disableRawCron: false,
      showInfo: true,
      cronFormatError: null,
      runAtInput: '',
    };
  },

  watch: {
    rawCron(val) {
      if (val) {
        localStorage.removeItem('schedule__raw_cron');
      } else {
        localStorage.setItem('schedule__raw_cron', '1');
      }
    },

    showInfo(val) {
      if (val) {
        localStorage.removeItem('schedule__hide_info');
      } else {
        localStorage.setItem('schedule__hide_info', '1');
      }
    },
  },

  async created() {
    this.showInfo = localStorage.getItem('schedule__hide_info') !== '1';
    this.rawCron = localStorage.getItem('schedule__raw_cron') !== '1';

    this.templates = (await axios({
      method: 'get',
      url: `/api/project/${this.projectId}/templates`,
      responseType: 'json',
    })).data;
  },

  props: {
    timezone: String,
    type: String,
  },

  computed: {
    MONTHS() {
      return Array.from({ length: 12 }, (_, i) => ({
        id: i + 1,
        title: new Intl.DateTimeFormat(this.$i18n.locale.replace('_', '-'), { month: 'long', timeZone: 'UTC' }).format(new Date(Date.UTC(2024, i, 1))),
      }));
    },
    WEEKDAYS() {
      return Array.from({ length: 7 }, (_, i) => ({
        id: i,
        title: new Intl.DateTimeFormat(this.$i18n.locale.replace('_', '-'), { weekday: 'short', timeZone: 'UTC' }).format(new Date(Date.UTC(2024, 0, 7 + i))),
      }));
    },
    localTimezone() {
      return this.$t('uiLocal');
    },

    runAtRules() {
      if (this.type === 'run_at') {
        return [];
      }

      return [
        (v) => !!v || this.$t('runTimeRequired'),
      ];
    },

    nextRunUtcDate() {
      return formatDateInTZ(this.nextRunTime(), this.timezone);
    },

    nextRunLocalDate() {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
      return formatDateInTZ(this.nextRunTime(), tz);
    },

    nextRunUtcTime() {
      return formatTimeInTZ(this.nextRunTime(), this.timezone);
    },

    nextRunLocalTime() {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
      return formatTimeInTZ(this.nextRunTime(), tz);
    },
  },

  methods: {
    getNewItem() {
      return {
        name: '',
        template_id: null,
        cron_format: '* * * * *',
        active: true,
        run_once: false,
        delete_after_run: false,
        task_params: {},
        run_at: null,
      };
    },

    setDefaultRunAt() {
      const nextHour = dayjs().tz(this.timezone).add(1, 'hour').minute(0)
        .second(0)
        .millisecond(0);

      this.runAtInput = nextHour.format(RUN_AT_FORMAT);
    },

    setRunAtInputFromItem() {
      if (!this.item.run_at) {
        this.runAtInput = '';
        return;
      }

      const parsed = dayjs(this.item.run_at).tz(this.timezone);
      this.runAtInput = parsed.isValid() ? parsed.format(RUN_AT_FORMAT) : '';
    },

    nextRunTime() {
      if (this.type === 'run_at') {
        const runAt = this.item.run_at ? dayjs(this.item.run_at) : null;
        const parsed = this.runAtInput
          ? dayjs.tz(this.runAtInput, RUN_AT_FORMAT, this.timezone)
          : runAt;

        if (!parsed || !parsed.isValid()) {
          return null;
        }

        return parsed.toDate();
      }

      try {
        return CronExpressionParser.parse(this.item.cron_format, {
          tz: this.timezone,
        }).next().toDate();
      } catch {
        return null;
      }
    },

    async validateCronFormat(cronFormat) {
      try {
        await axios({
          method: 'post',
          url: `/api/project/${this.projectId}/schedules/validate`,
          responseType: 'json',
          data: {
            project_id: this.projectId,
            cron_format: cronFormat,
          },
        });
        return null;
      } catch (err) {
        return err.response?.status === 400 ? this.$t('invalidCron') : getErrorMessage(err);
      }
    },

    async refreshCheckboxes() {
      if (this.type === 'run_at') {
        this.cronFormatError = null;
        this.disableRawCron = false;
        return;
      }

      // if (!/test/.test(this.item.cron_format)) {
      //   this.rawCron = true;
      //   this.disableRawCron = true;
      // } else {
      //   this.disableRawCron = false;
      // }

      this.cronFormatError = null;
      this.disableRawCron = false;

      const cronFormat = this.item.cron_format;
      const cronError = await this.validateCronFormat(cronFormat);

      if (cronFormat !== this.item.cron_format) {
        return; // the value changed while validating, ignore stale result
      }

      if (cronError != null) {
        this.cronFormatError = cronError;
        this.rawCron = true;
        this.disableRawCron = true;
        return;
      }

      let cron;
      try {
        cron = CronExpressionParser.parse(this.item.cron_format, {
          tz: this.timezone,
        });
      } catch {
        // Valid on the backend but not parseable by cron-parser
        // (e.g. @hourly) — show it in raw mode without an error.
        this.rawCron = true;
        this.disableRawCron = true;
        return;
      }

      const fields = cron.fields; // JSON.parse(JSON.stringify(cron.fields));

      this.months = [];
      this.weekdays = [];
      this.hours = [];
      this.minutes = [];

      if (this.isHourly(this.item.cron_format)) {
        this.minutes = fields.minute.values;
        this.timing = 'hourly';
      } else {
        this.minutes = [];
      }

      if (this.isDaily(this.item.cron_format)) {
        this.hours = fields.hour.values;
        this.timing = 'daily';
      } else {
        this.hours = [];
      }

      if (this.isWeekly(this.item.cron_format)) {
        this.weekdays = fields.dayOfWeek.values;
        this.timing = 'weekly';
      } else {
        this.weekdays = [];
      }

      if (this.isMonthly(this.item.cron_format)) {
        this.days = fields.dayOfMonth.values;
        this.timing = 'monthly';
      } else {
        this.months = [];
      }

      if (this.isYearly(this.item.cron_format)) {
        this.months = fields.month.values;
        this.timing = 'yearly';
      }
    },

    afterLoadData() {
      // if (!this.item.type) {
      //   this.item.type = this.item.run_at ? 'run_at' : '';
      // }

      if (this.item.run_at) {
        this.setRunAtInputFromItem();
      } else if (this.type === 'run_at') {
        this.setDefaultRunAt();
      } else if (this.isNew) {
        this.item.cron_format = '* * * * *';
      }

      this.refreshCheckboxes();
    },

    async beforeSave() {
      this.item.type = this.type;

      if (this.type === 'run_at') {
        const parsed = this.runAtInput
          ? dayjs.tz(this.runAtInput, RUN_AT_FORMAT, this.timezone)
          : null;

        if (!parsed || !parsed.isValid()) {
          this.formError = this.$t('runTimeInvalid');
          throw new Error(this.formError);
        }

        this.item.run_at = parsed.toISOString();
        this.item.cron_format = this.item.cron_format || '';
      } else {
        this.item.run_at = null;
      }
    },

    isWeekly(s) {
      return isWeekly(s);
    },

    isYearly(s) {
      return isYearly(s);
    },

    isMonthly(s) {
      return isMonthly(s);
    },

    isDaily(s) {
      return isDaily(s);
    },

    isHourly(s) {
      return isHourly(s);
    },

    refreshCron() {
      const selections = pruneSelectionsForTiming(this.timing, {
        months: this.months,
        weekdays: this.weekdays,
        days: this.days,
        hours: this.hours,
        minutes: this.minutes,
      });

      this.months = selections.months;
      this.weekdays = selections.weekdays;
      this.days = selections.days;
      this.hours = selections.hours;
      this.minutes = selections.minutes;

      this.item.cron_format = buildCronFormat(selections);
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/schedules`;
    },

    getSingleItemUrl() {
      return `/api/project/${this.projectId}/schedules/${this.itemId}`;
    },

  },
};
</script>
