<template>
  <div class="pa-4">
    <h3 class="mb-3">Job Executor Hub</h3>
    <p>JEH includes the open-source implementation available in this repository.</p>
    <p>Proprietary upstream Pro and Enterprise features are not included in this release.</p>
    <v-btn
      color="primary"
      href="https://github.com/impishMD/jeh/blob/develop/docs/en/configuration.md#feature-scope"
      target="_blank"
      rel="noopener noreferrer"
    >
      Feature availability
    </v-btn>
  </div>
</template>

<script>
export default {
  props: {
    feature: String,
  },
};
</script>
