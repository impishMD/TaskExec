<template>
  <v-combobox ref="path" :value="value" :search-input.sync="search" :items="items"
    :label="$t('vaultSecretPath')" :loading="loading" :disabled="disabled"
    :rules="[(v) => !!v || $t('required'),
      (v) => !(v || '').includes('#') || $t('vaultPathNoField')]"
    :return-object="false" no-filter outlined dense clearable
    @input="selectPath" @input.native="openMenu" @click:clear="clearPath" @focus="focusPath">
    <template v-slot:item="{ item }">
      <v-icon small class="mr-2">
        {{ item.endsWith('/') ? 'mdi-folder-outline' : 'mdi-key-outline' }}
      </v-icon>
      {{ item }}
    </template>
    <template v-slot:no-data>
      <v-list-item><v-list-item-content class="text-body-2">
        {{ error || $t('noValues') }}
      </v-list-item-content></v-list-item>
    </template>
  </v-combobox>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';

export default {
  props: {
    value: String,
    projectId: [Number, String],
    storageId: [Number, String],
    disabled: Boolean,
  },
  data() {
    return {
      search: this.value || '',
      paths: [],
      cache: {},
      loading: false,
      error: null,
      timer: null,
      request: 0,
    };
  },
  computed: {
    directory() {
      const path = (this.search || '').replace(/^\/+/, '');
      return path.slice(0, path.lastIndexOf('/') + 1);
    },
    location() { return `${this.projectId}:${this.storageId}:${this.directory}`; },
    items() {
      const prefix = (this.search || '').replace(/^\/+/, '').toLowerCase();
      return this.paths.filter((path) => path.toLowerCase().startsWith(prefix));
    },
  },
  watch: {
    value(value) { if (value !== this.search) this.search = value || ''; },
    search(value) { if (value != null) this.setPath(value); },
    location() {
      this.request += 1;
      this.paths = [];
      this.error = null;
      this.loading = false;
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.loadPaths(), 200);
    },
  },
  mounted() { this.loadPaths(); },
  beforeDestroy() { clearTimeout(this.timer); this.request += 1; },
  methods: {
    async openMenu() {
      await this.$nextTick();
      if (this.$refs.path?.isFocused) this.$refs.path.isMenuActive = true;
    },
    focusPath() { this.loadPaths(); this.openMenu(); },
    clearPath() { this.search = ''; this.setPath(''); this.openMenu(); },
    setPath(path) {
      if (path !== (this.value || '')) {
        this.$emit('input', path);
        this.$emit('change', path);
      }
    },
    async selectPath(path) {
      if (typeof path !== 'string') return;
      this.search = path;
      this.setPath(path);
      if (path.endsWith('/')) {
        await this.$nextTick();
        this.$refs.path.focus();
        this.$refs.path.isMenuActive = true;
      }
    },
    async loadPaths() {
      if (!this.storageId || this.disabled || this.loading) return;
      const location = this.location;
      if (this.cache[location]) { this.paths = this.cache[location]; return; }
      this.request += 1;
      const request = this.request;
      this.loading = true;
      this.error = null;
      try {
        const { data } = await axios.post(
          `/api/project/${this.projectId}/secret_storages/${this.storageId}/paths`,
          { path: this.directory },
        );
        if (request !== this.request) return;
        this.paths = data.paths;
        this.cache[location] = data.paths;
      } catch (err) {
        if (request === this.request) this.error = getErrorMessage(err);
      } finally {
        if (request === this.request) this.loading = false;
      }
    },
  },
};
</script>
