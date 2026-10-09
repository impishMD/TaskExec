<template>
  <v-text-field ref="input" :value="value" v-bind="$attrs" v-on="listeners"
    @input="input" @click="rememberCaret" @keyup="rememberCaret">
    <template v-slot:append v-if="options.length">
      <v-menu v-model="open" offset-y :max-height="280" :max-width="520" :min-width="280">
        <template v-slot:activator="{ on, attrs }">
          <v-btn icon small v-bind="attrs" v-on="on" :aria-label="$t('keyInsertReference')"
            @mousedown.prevent="rememberCaret">
            <v-icon small>mdi-code-braces</v-icon>
          </v-btn>
        </template>
        <v-list dense>
          <v-list-item v-for="option in filtered" :key="option" @click="insert(option)">
            <v-list-item-icon class="mr-3"><v-icon small>mdi-key-outline</v-icon></v-list-item-icon>
            <v-list-item-content><code>{{ option }}</code></v-list-item-content>
          </v-list-item>
          <v-list-item v-if="!filtered.length">{{ $t('noValues') }}</v-list-item>
        </v-list>
      </v-menu>
    </template>
  </v-text-field>
</template>

<script>
export default {
  inheritAttrs: false,
  props: {
    value: [String, Number, Boolean], options: { type: Array, default: () => [] },
  },
  data: () => ({ open: false, caret: null, query: '' }),
  computed: {
    listeners() { const { input, ...rest } = this.$listeners; return rest; },
    filtered() {
      return this.options.filter(
        (option) => option.toLowerCase().includes(this.query.toLowerCase()),
      );
    },
  },
  methods: {
    rememberCaret() {
      const input = this.$refs.input?.$el.querySelector('input');
      if (input && document.activeElement === input) this.caret = input.selectionStart;
    },
    input(value) {
      this.$emit('input', value);
      this.rememberCaret();
      const fragment = String(value || '').slice(0, this.caret).match(/\{\{([^}]*)$/);
      this.query = fragment ? fragment[1].trim() : '';
      this.open = !!fragment && this.options.length > 0;
    },
    async insert(expression) {
      const value = String(this.value == null ? '' : this.value);
      const caret = this.caret == null ? value.length : this.caret;
      const before = value.slice(0, caret); const
        fragment = before.match(/\{\{[^}]*$/);
      const start = fragment ? before.length - fragment[0].length : caret;
      const tail = value.slice(caret).replace(fragment ? /^\s*\}\}/ : /$^/, '');
      const next = value.slice(0, start) + expression + tail;
      this.$emit('input', next);
      this.open = false; this.query = '';
      await this.$nextTick();
      const input = this.$refs.input.$el.querySelector('input');
      input.focus(); input.setSelectionRange(start + expression.length, start + expression.length);
      this.caret = start + expression.length;
    },
  },
};
</script>
