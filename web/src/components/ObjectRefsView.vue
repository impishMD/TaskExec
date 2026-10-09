<template>
  <div class="object-refs-view">
    <v-alert
      type="warning"
    >
      {{ $t('theCantBeDeletedBecauseItUsedByTheResourcesBelow', {objectTitle: objectTitle}) }}
    </v-alert>
    <div
      v-for="s in sections"
      class="object-refs-view__section"
      :key="s.slug"
    >
      <div class="object-refs-view__section-title">
        <v-icon small class="mr-2">mdi-{{ s.icon }}</v-icon>{{ s.title }}:
      </div>

      <div class="ml-6">
        <span
            v-for="t in (objectRefs[s.slug] || [])"
            class="object-refs-view__link-wrap"
            :key="t.id"
        >
          <router-link
            :to="`/project/${projectId}/${s.path || s.slug}/${s.pageless ? '' : t.id}`"
            class="object-refs-view__link">{{ t.name }}</router-link>
        </span>
      </div>
    </div>
  </div>
</template>
<style lang="scss">
.object-refs-view__section {
  margin-bottom: 10px;
}

.object-refs-view__link-wrap + .object-refs-view__link-wrap {
  &:before {
    content: ", ";
  }
}
</style>
<script>
export default {
  props: {
    objectRefs: Object,
    projectId: Number,
    objectTitle: String,
  },
  computed: {
    sections() {
      return [{
        slug: 'templates',
        title: this.$t('uiTemplates'),
        icon: 'check-all',
      }, {
        slug: 'workflows',
        title: this.$t('workflows'),
        icon: 'sitemap',
      }, {
        slug: 'inventories',
        title: this.$t('uiInventories'),
        icon: 'monitor-multiple',
      }, {
        slug: 'repositories',
        title: this.$t('repositories'),
        icon: 'git',
      }, {
        slug: 'integrations',
        title: this.$t('integrations'),
        icon: 'connection',
      }, {
        slug: 'access_keys',
        pageless: true,
        path: 'keys',
        title: this.$t('uiAccessKeys'),
        icon: 'key-change',
      }, {
        slug: 'environments',
        path: 'environment',
        pageless: true,
        title: this.$t('environment'),
        icon: 'code-braces',
      }, {
        slug: 'schedules',
        title: this.$t('uiSchedules'),
        icon: 'clock-outline',
      }, {
        slug: 'host_configs',
        path: 'host_config',
        pageless: true,
        title: this.$t('hostConfig'),
        icon: 'server-network',
      }].filter((s) => (this.objectRefs[s.slug] || []).length > 0);
    },
  },
};
</script>
