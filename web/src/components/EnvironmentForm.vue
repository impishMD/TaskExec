<template>
  <v-form
    ref="form"
    lazy-validation
    v-model="formValid"
    v-if="item != null"
    class="pb-3"
  >
    <v-alert :value="!!formError" color="error" data-testid="varGroup-error"
      >{{ formError }}
    </v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t('environmentName')"
      :rules="[(v) => !!v || $t('name_required')]"
      required
      :disabled="formSaving"
      outlined
      dense
    ></v-text-field>

    <KeySourcesForm v-model="item.key_sources" :project-id="projectId"
      :disabled="formSaving" @options="expressionOptions = $event" />

    <v-tabs grow v-model="tab">
      <v-tab key="variables">{{ $t('uiVariables') }}</v-tab>
      <v-tab key="secrets">{{ $t('uiSecrets') }}</v-tab>
      <v-tab key="key-bindings">{{ $t('keyBindings') }}</v-tab>
    </v-tabs>

    <v-divider style="margin-top: -1px" class="mb-7" />

    <v-tabs-items v-model="tab">
      <v-tab-item key="variables">
        <v-subheader class="px-0">
          {{ $t('extraVariables') }}

          <HelpHint :visible="needHelp" class="ml-1">
            <div>
              <div><code>--extra-vars</code> {{ $t('forApp', { app: 'Ansible' }) }}</div>
              <div><code>-var</code> {{ $t('forApp', { app: 'Terraform/OpenTofu' }) }}</div>
            </div>
          </HelpHint>

          <v-spacer />

          <v-btn-toggle :key="modeRevision" v-model="extraVarsEditMode" tile group mandatory>
            <v-btn value="table" small class="mr-0" style="border-radius: 4px">
              {{ $t('uiTable') }}
            </v-btn>
            <v-btn value="json" small class="mr-0" style="border-radius: 4px"> JSON </v-btn>
            <v-btn value="yaml" small class="mr-0" style="border-radius: 4px"> YAML </v-btn>
          </v-btn-toggle>

          <v-btn icon @click="addExtraVar()" data-testid="varGroup-addVar">
            <v-icon> mdi-plus </v-icon>
          </v-btn>
        </v-subheader>

        <div key="json" v-if="extraVarsEditMode === 'json'" style="position: relative">
          <codemirror
            :class="{
              EnvironmentEditor: true,
            }"
            :style="{ border: '1px solid lightgray' }"
            v-model="json"
            :options="cmOptions"
            :placeholder="$t('enterExtraVariablesJson')"
          />

          <RichEditor
            v-model="json"
            type="json"
            v-if="extraVarsEditMode === 'json'"
            style="position: absolute; right: 10px; top: 0; margin: 10px"
          />
        </div>
        <div key="yaml" v-else-if="extraVarsEditMode === 'yaml'" style="position: relative">
          <codemirror
            :class="{
              EnvironmentEditor: true,
            }"
            :style="{ border: '1px solid lightgray' }"
            v-model="yaml"
            :options="cmYamlOptions"
            :placeholder="$t('enterExtraVariablesYaml')"
          />

          <RichEditor
            v-model="yaml"
            type="yaml"
            v-if="extraVarsEditMode === 'yaml'"
            style="position: absolute; right: 10px; top: 0; margin: 10px"
          />
        </div>
        <div v-else-if="extraVarsEditMode === 'table'">
          <v-data-table
            v-if="extraVars != null"
            :items="extraVars"
            :items-per-page="-1"
            class="elevation-1 FieldTable"
            hide-default-footer
            :no-data-text="$t('noValues')"
            style="background: #8585850f"
          >
            <template v-slot:item="props">
              <tr>
                <td class="pa-1">
                  <v-text-field
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.name"
                    class="v-text-field--solo--no-min-height"
                    :placeholder="$t('name')"
                  ></v-text-field>
                </td>
                <td class="pa-1" style="width: 130px">
                  <v-select
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.type"
                    :items="extraVarTypes"
                    class="v-text-field--solo--no-min-height"
                    data-testid="varGroup-varType"
                  ></v-select>
                </td>
                <td class="pa-1">
                  <div class="d-flex align-center">
                    <KeyExpressionInput
                      solo-inverted
                      flat
                      hide-details
                      v-model="props.item.value"
                      class="v-text-field--solo--no-min-height"
                      :placeholder="extraVarValuePlaceholder(props.item.type)"
                     :options="expressionOptions" />
                    <RichEditor
                      v-if="props.item.type === 'list' || props.item.type === 'dict'"
                      v-model="props.item.value"
                      :type="props.item.type === 'list' ? 'json_array' : 'json'"
                      class="ml-1 EnvVarExpandBtn"
                    />
                  </div>
                </td>
                <td style="width: 38px">
                  <v-icon small class="pa-1" @click="removeExtraVar(props.item)">
                    mdi-delete
                  </v-icon>
                </td>
              </tr>
            </template>
          </v-data-table>

          <v-alert color="warning" v-else>
            {{ $t('uiOopsThisJSONStructureIsALittleTooComplexToDisplayAsATable') }}
          </v-alert>
        </div>

        <div>
          <v-subheader class="px-0 mt-4">
            {{ $t('environmentVariables') }}

            <v-spacer />

            <v-btn icon @click="addEnvVar()" data-testid="varGroup-addEnv">
              <v-icon> mdi-plus </v-icon>
            </v-btn>
          </v-subheader>
          <v-data-table
            :items="env"
            :items-per-page="-1"
            class="elevation-1 FieldTable"
            hide-default-footer
            :no-data-text="$t('noValues')"
            style="background: #8585850f"
          >
            <template v-slot:item="props">
              <tr>
                <td class="pa-1">
                  <v-text-field
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.name"
                    class="v-text-field--solo--no-min-height"
                    :placeholder="$t('name')"
                  ></v-text-field>
                </td>
                <td class="pa-1">
                  <KeyExpressionInput
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.value"
                    class="v-text-field--solo--no-min-height"
                    :placeholder="$t('matchValue')"
                   :options="expressionOptions" />
                </td>
                <td style="width: 38px">
                  <v-icon small class="pa-1" @click="removeEnvVar(props.item)"> mdi-delete </v-icon>
                </td>
              </tr>
            </template>
          </v-data-table>
        </div>
      </v-tab-item>

      <v-tab-item key="secrets">

        <div>
          <v-subheader class="px-0">
            {{ $t('extraVariables') }}
            <HelpHint :visible="needHelp" class="ml-1">
              <div>
                <div><code>--extra-vars</code> {{ $t('forApp', { app: 'Ansible' }) }}</div>
                <div><code>-var</code> {{ $t('forApp', { app: 'Terraform/OpenTofu' }) }}</div>
              </div>
            </HelpHint>

            <v-spacer />
            <v-btn icon @click="addSecret('var')" data-testid="varGroup-addSecretVar">
              <v-icon> mdi-plus </v-icon>
            </v-btn>
          </v-subheader>

          <v-alert
            color="warning"
            text
            v-if="secrets.filter((s) => !s.remove && s.type === 'var').length > 0"
          >{{ $t('uiSecretsPassedThisWayMayAppearInPlainTextInAnsibleLogs') }}</v-alert>

          <v-data-table
            :items="secrets.filter((s) => !s.remove && s.type === 'var')"
            :items-per-page="-1"
            class="elevation-1 FieldTable"
            hide-default-footer
            :no-data-text="$t('noValues')"
            style="background: #8585850f"
          >
            <template v-slot:item="props">
              <tr>
                <td class="pa-1">
                  <v-text-field
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.name"
                    class="v-text-field--solo--no-min-height"
                    :placeholder="$t('name')"
                  ></v-text-field>
                </td>

                <td class="pa-1">
                  <KeyExpressionInput
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.value"
                    placeholder="*******"
                    class="v-text-field--solo--no-min-height"
                   :options="expressionOptions" />
                </td>

                <td style="width: 38px">
                  <v-icon small class="pa-1" @click="removeSecret(props.item)"> mdi-delete </v-icon>
                </td>
              </tr>
            </template>
          </v-data-table>
        </div>

        <div>
          <v-subheader class="px-0 mt-4">
            {{ $t('environmentVariables') }}

            <v-spacer />

            <v-btn icon @click="addSecret('env')" data-testid="varGroup-addSecretEnv">
              <v-icon> mdi-plus </v-icon>
            </v-btn>
          </v-subheader>

          <v-data-table
            :items="secrets.filter((s) => !s.remove && s.type === 'env')"
            :items-per-page="-1"
            class="elevation-1 FieldTable"
            hide-default-footer
            :no-data-text="$t('noValues')"
            style="background: #8585850f"
          >
            <template v-slot:item="props">
              <tr>
                <td class="pa-1">
                  <v-text-field
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.name"
                    class="v-text-field--solo--no-min-height"
                    :placeholder="$t('name')"
                  ></v-text-field>
                </td>

                <td class="pa-1">
                  <KeyExpressionInput
                    solo-inverted
                    flat
                    hide-details
                    v-model="props.item.value"
                    placeholder="*******"
                    class="v-text-field--solo--no-min-height"
                   :options="expressionOptions" />
                </td>

                <td style="width: 38px">
                  <v-icon small class="pa-1" @click="removeSecret(props.item)"> mdi-delete </v-icon>
                </td>
              </tr>
            </template>
          </v-data-table>
        </div>
      </v-tab-item>
      <v-tab-item key="key-bindings" eager>
        <KeyBindingsForm v-model="item.key_bindings"
          :project-id="projectId" :disabled="formSaving" />
      </v-tab-item>
    </v-tabs-items>
  </v-form>
</template>
<style lang="scss">
.EnvironmentEditor {
  .CodeMirror {
    height: 160px !important;
  }
}

// Compact the RichEditor "expand" fab so it fits inside a variables table row.
.EnvVarExpandBtn {
  .v-btn--fab.v-size--small {
    height: 30px;
    width: 30px;
  }

  .v-btn__content .v-icon {
    font-size: 18px;
  }
  position: absolute;
  right: 68px;
}
</style>
<script>
/* eslint-disable import/no-extraneous-dependencies,import/extensions */

import ItemFormBase from '@/components/ItemFormBase';
import HelpHint from '@/components/HelpHint.vue';

import { codemirror } from 'vue-codemirror';
import {
  dump as dumpYaml,
} from 'js-yaml';
import {
  isPlainObject,
  isJsonSafeValue,
  inferVarType,
  rowToVarValue,
  extraVarsToObject,
  extraVarsToObjectLenient,
  objectToExtraVars,
  parseExtraVars,
} from '@/lib/extraVars';
import 'codemirror/lib/codemirror.css';
import 'codemirror/mode/vue/vue.js';
import 'codemirror/mode/yaml/yaml.js';
import 'codemirror/addon/display/placeholder.js';
import { getErrorMessage } from '@/lib/error';
import RichEditor from '@/components/RichEditor.vue';
import KeyBindingsForm from '@/components/KeyBindingsForm.vue';
import KeySourcesForm from '@/components/KeySourcesForm.vue';
import KeyExpressionInput from '@/components/KeyExpressionInput.vue';

export default {
  mixins: [ItemFormBase],

  props: {
    needHelp: Boolean,
  },

  components: {
    HelpHint,
    RichEditor,
    KeyBindingsForm,
    codemirror,
    KeySourcesForm,
    KeyExpressionInput,
  },

  computed: {
    extraVarsEditMode: {
      get() { return this.editorMode; },
      set(mode) { this.switchExtraVarsMode(mode); },
    },

  },

  data() {
    return {
      expressionOptions: [],
      json: '{}',
      yaml: '',
      extraVars: [],
      env: [],
      secrets: [],

      tab: 'variables',

      cmOptions: {
        tabSize: 2,
        mode: 'application/json',
        lineNumbers: true,
        line: true,
        lint: true,
        indentWithTabs: false,
      },

      cmYamlOptions: {
        tabSize: 2,
        mode: 'text/x-yaml',
        lineNumbers: true,
        line: true,
        indentWithTabs: false,
      },

      editorMode: 'table',
      modeRevision: 0,

      extraVarTypes: [
        { text: this.$t('uiString'), value: 'string' },
        { text: this.$t('uiNumber'), value: 'number' },
        { text: this.$t('uiList'), value: 'list' },
        { text: this.$t('uiDict'), value: 'dict' },
      ],

    };
  },

  methods: {
    afterReset() {
      this.editorMode = 'table';
      this.modeRevision += 1;
      this.json = '{}';
      this.yaml = '';
      this.extraVars = [];
      this.env = [];
      this.secrets = [];
      this.tab = 'variables';
      this.expressionOptions = [];
    },
    getNewItem() { return { key_sources: [], key_bindings: [], secret_expressions: [] }; },
    isExpression(value) {
      if (typeof value !== 'string') return false;
      return [...value.matchAll(/\{\{\s*([A-Za-z_][A-Za-z0-9_]*)/g)]
        .some((match) => (this.item.key_sources || [])
          .some((source) => source.prefix === match[1]));
    },

    addExtraVar(name = '', value = '', type = 'string') {
      this.extraVars.push({ name, value, type });
    },

    removeExtraVar(val) {
      const i = this.extraVars.indexOf(val);
      if (i > -1) {
        this.extraVars.splice(i, 1);
      }
    },

    extraVarValuePlaceholder(type) {
      switch (type) {
        case 'number':
          return '42';
        case 'list':
          return '["a", "b"]';
        case 'dict':
          return '{"key": "value"}';
        default:
          return this.$t('matchValue');
      }
    },

    // The conversions below live in @/lib/extraVars; the methods are kept so
    // the template and the watchers can keep calling them on `this`.
    isPlainObject(value) {
      return isPlainObject(value);
    },

    isJsonSafeValue(value, seen) {
      return isJsonSafeValue(value, seen);
    },

    readExtraVars() {
      return this.editorMode === 'table'
        ? this.extraVarsToObjectLenient(this.extraVars)
        : parseExtraVars(this[this.editorMode], this.editorMode);
    },

    switchExtraVarsMode(mode) {
      if (!['table', 'json', 'yaml'].includes(mode) || mode === this.editorMode) return;
      try {
        const source = this.readExtraVars();
        if (!this.isJsonSafeValue(source)) throw new Error(this.$t('extraVarsFiniteRequired'));
        if (mode === 'json') this.json = JSON.stringify(source, null, 2);
        if (mode === 'yaml') this.yaml = dumpYaml(source);
        if (mode === 'table'
          && JSON.stringify(source)
          !== JSON.stringify(this.extraVarsToObjectLenient(this.extraVars))) {
          this.extraVars = this.objectToExtraVars(source);
        }
        this.editorMode = mode;
        this.formError = null;
      } catch (err) {
        this.formError = getErrorMessage(err);
        this.modeRevision += 1;
      }
    },

    inferVarType(value) {
      return inferVarType(value);
    },

    rowToVarValue(row) {
      return rowToVarValue(row);
    },

    extraVarsToObject(rows) {
      return extraVarsToObject(rows);
    },

    extraVarsToObjectLenient(rows) {
      return extraVarsToObjectLenient(rows);
    },

    objectToExtraVars(obj) {
      return objectToExtraVars(obj);
    },

    addEnvVar(name = '', value = '') {
      this.env.push({ name, value });
    },

    removeEnvVar(val) {
      const i = this.env.indexOf(val);
      if (i > -1) {
        this.env.splice(i, 1);
      }
    },

    addSecret(type) {
      this.secrets.push({
        type,
        name: '',
        value: '',
        new: true,
      });
    },

    removeSecret(val) {
      const i = this.secrets.indexOf(val);
      if (i > -1) {
        const s = this.secrets[i];
        this.secrets.splice(i, 1);

        if (!s.new) {
          this.secrets.push({
            ...s,
            remove: true,
          });
        }
      }
    },

    beforeSave() {
      this.item.json = JSON.stringify(this.editorMode === 'table'
        ? this.extraVarsToObject(this.extraVars)
        : parseExtraVars(this[this.editorMode], this.editorMode));

      const env = (this.env || []).reduce(
        (prev, curr) => ({
          ...prev,
          [curr.name]: curr.value,
        }),
        {},
      );

      const secrets = [];
      const expressions = [];
      (this.secrets || []).forEach((secret) => {
        const mapped = this.isExpression(secret.value);
        if (secret.id && (secret.remove || mapped)) {
          secrets.push({
            id: secret.id, name: secret.name, type: secret.type, operation: 'delete',
          });
        }
        if (secret.remove) return;
        if (mapped) {
          expressions.push({ name: secret.name, type: secret.type, expression: secret.value });
          return;
        }
        if (secret.expression && /\{\{/.test(secret.value)) {
          const error = new Error(this.$t('keyExpressionInvalid'));
          throw error;
        }
        secrets.push({
          id: secret.id,
          name: secret.name,
          secret: secret.value,
          type: secret.type,
          operation: secret.id ? 'update' : 'create',
        });
      });
      this.item.secret_expressions = expressions;
      this.item.key_sources = (this.item.key_sources || []).map((source) => ({
        prefix: source.prefix, key_id: source.key_id,
      }));
      this.item.key_bindings = (this.item.key_bindings || []).map((b) => ({
        key_id: b.key_id, name: b.name, type: b.type, field: b.field == null ? null : b.field,
      }));
      this.item.env = JSON.stringify(env);
      this.item.secrets = secrets;
    },

    async afterLoadData() {
      this.$set(this.item, 'key_bindings', this.item.key_bindings || []);
      this.$set(this.item, 'key_sources', this.item.key_sources || []);

      this.json = JSON.stringify(JSON.parse(this.item?.json || '{}'), null, 2);

      const json = JSON.parse(this.item?.json || '{}');

      const env = JSON.parse(this.item?.env || '{}');

      const secrets = this.item?.secrets || [];

      this.extraVars = this.objectToExtraVars(json);
      this.yaml = dumpYaml(json);
      this.editorMode = 'table';
      this.tab = 'variables';
      this.formError = null;

      this.env = Object.keys(env)
        .map((x) => ({
          name: x,
          value: env[x],
        }));

      this.secrets = secrets.map((x) => ({
        id: x.id,
        name: x.name,
        value: '',
        type: x.type,
      }));

      this.secrets.push(...(this.item.secret_expressions || []).map((expression) => ({
        name: expression.name,
        type: expression.type,
        value: expression.expression,
        expression: true,
      })));
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/environment`;
    },

    getSingleItemUrl() {
      return `/api/project/${this.projectId}/environment/${this.itemId}`;
    },
  },
};
</script>
