// Each page or dialog owns its help mode; nested dialogs get independent scopes.
export default {
  data: () => ({ contextHelp: { enabled: false } }),
  provide() { return { contextHelp: this.contextHelp }; },
  computed: {
    needHelp: {
      get() { return this.contextHelp.enabled; },
      set(value) { this.contextHelp.enabled = value; },
    },
  },
};
