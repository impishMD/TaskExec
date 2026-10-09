<template>
  <div class="project-icon-picker mb-6">
    <div class="text-body-2 text--secondary mb-3">{{ $t('projectIcon') }}</div>
    <div class="project-icon-picker__controls">
      <ProjectAvatar :project="{ name, icon: value }" :size="56" />
      <v-menu v-model="menu" offset-y :close-on-content-click="false" max-width="320"
        content-class="project-icon-menu">
        <template v-slot:activator="{ on, attrs }">
          <v-btn outlined color="primary" :disabled="disabled || loading" v-on="on" v-bind="attrs">
            {{ $t('projectIconChoose') }}
          </v-btn>
        </template>
        <v-card class="pa-3 project-icon-grid">
          <v-btn v-for="(icon, index) in icons" :key="icon" icon
            :color="icon === value ? 'primary' : undefined" :aria-pressed="String(icon === value)"
            :aria-label="$t('projectIconSymbol', { number: index + 1 })" @click="select(icon)">
            <v-icon>{{ icon }}</v-icon>
          </v-btn>
        </v-card>
      </v-menu>
      <v-btn text color="primary" :disabled="disabled" :loading="loading"
        @click="$refs.file.click()">{{ $t('projectIconUpload') }}</v-btn>
      <v-btn v-if="value" icon :disabled="disabled || loading" :aria-label="$t('projectIconRemove')"
        @click="select(null)"><v-icon>mdi-delete-outline</v-icon></v-btn>
      <input ref="file" type="file" accept="image/png,image/jpeg,image/webp" hidden
        :aria-label="$t('projectIconUpload')" @change="upload" />
    </div>
    <div class="text-caption text--secondary mt-2">{{ $t('projectIconFormats') }}</div>
    <v-alert v-if="error" type="error" text dense class="mt-3 mb-0">{{ error }}</v-alert>
  </div>
</template>
<script>
import ProjectAvatar from '@/components/ProjectAvatar.vue';
import { projectIcons, readProjectIcon } from '@/lib/projectIcon';

export default {
  components: { ProjectAvatar },
  props: { value: String, name: String, disabled: Boolean },
  data: () => ({
    icons: projectIcons, menu: false, loading: false, error: null, active: true,
  }),
  beforeDestroy() { this.active = false; },
  methods: {
    select(icon) { this.$emit('input', icon); this.error = null; this.menu = false; },
    async upload(event) {
      const [file] = event.target.files;
      const input = event.target;
      input.value = '';
      if (!file) return;
      this.loading = true;
      this.error = null;
      this.$emit('loading', true);
      try {
        const icon = await readProjectIcon(file);
        if (this.active) this.select(icon);
      } catch (err) {
        if (this.active) {
          this.error = this.$t(err.message === 'projectIconFormats'
            ? err.message : 'projectIconInvalid');
        }
      } finally {
        if (this.active) { this.loading = false; this.$emit('loading', false); }
      }
    },
  },
};
</script>
<style lang="scss">
.project-icon-picker__controls { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.project-icon-grid { display: grid; grid-template-columns: repeat(6, 40px); gap: 4px; }
.project-icon-menu { max-width: calc(100vw - 32px) !important; }
@media (max-width: 360px) { .project-icon-grid { grid-template-columns: repeat(5, 40px); } }
</style>
