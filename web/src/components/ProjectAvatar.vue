<template>
  <v-avatar :color="imageIcon ? undefined : color" :size="size"
    class="project-avatar" aria-hidden="true">
    <img v-if="imageIcon" :src="project.icon" alt="" @error="failed = true" />
    <v-icon v-else-if="symbolIcon" :size="Number(size) * .68" color="white">
      {{ project.icon }}
    </v-icon>
    <span v-else class="white--text" :style="{ fontSize: `${Number(size) * .54}px` }">
      {{ initials }}
    </span>
  </v-avatar>
</template>
<script>
import { projectInitials } from '@/lib/projectIcon';

export default {
  props: {
    project: { type: Object, required: true },
    size: { type: [Number, String], default: 24 },
    color: { type: String, default: 'orange' },
  },
  data: () => ({ failed: false }),
  computed: {
    imageIcon() { return !this.failed && this.project.icon?.startsWith('data:image/png;base64,'); },
    symbolIcon() { return /^mdi-[a-z0-9]+(?:-[a-z0-9]+)*$/.test(this.project.icon || ''); },
    initials() { return projectInitials(this.project.name); },
  },
  watch: { 'project.icon': function reset() { this.failed = false; } },
};
</script>
<style lang="scss">
.project-avatar {
  flex-shrink: 0;
  font-weight: bold;
  img { object-fit: contain; }
}
</style>
