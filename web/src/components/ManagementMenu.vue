<template>
  <v-menu top offset-y min-width="260" max-width="300" content-class="taskexec-management-menu">
    <template v-slot:activator="{ on, attrs }">
      <v-btn
        icon
        v-bind="attrs"
        v-on="on"
        :aria-label="$t('management')"
        :title="$t('management')"
        :class="{ 'taskexec-management-active': active }"
        data-testid="sidebar-management"
      >
        <v-icon>mdi-cog-outline</v-icon>
      </v-btn>
    </template>

    <v-list dense data-testid="management-menu">
      <v-subheader>{{ $t('management') }}</v-subheader>
      <template v-if="isAdmin">
        <v-list-item
          v-for="item in adminItems"
          :key="item.to"
          :to="item.to"
          :data-testid="`management-${item.id}`"
        >
          <v-list-item-icon><v-icon>{{ item.icon }}</v-icon></v-list-item-icon>
          <v-list-item-title>{{ $t(item.title) }}</v-list-item-title>
        </v-list-item>

        <v-divider class="my-1" />

        <v-list-item @click="$emit('system-info')" data-testid="management-system-info">
          <v-list-item-icon><v-icon>mdi-information-outline</v-icon></v-list-item-icon>
          <v-list-item-title>{{ $t('systemInfo') }}</v-list-item-title>
        </v-list-item>
      </template>

      <v-subheader v-if="!isAdmin && version">TaskExec {{ version }}</v-subheader>
    </v-list>
  </v-menu>
</template>

<script>
export default {
  props: {
    isAdmin: Boolean,
    active: Boolean,
    version: String,
  },

  computed: {
    adminItems() {
      const items = [
        {
          id: 'settings', to: '/settings', icon: 'mdi-tune', title: 'generalSettings',
        },
        {
          id: 'alerts', to: '/alerts', icon: 'mdi-bell-outline', title: 'alertsTitle',
        },
        {
          id: 'users', to: '/users', icon: 'mdi-account-multiple-outline', title: 'users',
        },
        {
          id: 'runners', to: '/runners', icon: 'mdi-cogs', title: 'runners',
        },
        {
          id: 'tasks', to: '/tasks', icon: 'mdi-play-circle-outline', title: 'activeTasks',
        },
        {
          id: 'apps', to: '/apps', icon: 'mdi-apps', title: 'applications',
        },
      ];
      items.push({
        id: 'roles', to: '/roles', icon: 'mdi-account-cog-outline', title: 'Roles',
      });
      return items;
    },
  },
};
</script>
