<template>
  <v-card
    style="background: var(--highlighted-card-bg-color)"
    class="mb-6"
  >

    <v-card-text :class="{'pb-0': noBottomPadding}">
      <slot />
    </v-card-text>
  </v-card>
</template>

<script>
export default {
  props: {
    noBottomPadding: Boolean,
  },
};
</script>
