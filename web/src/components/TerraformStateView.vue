<template>
    <pre v-if="state" class="terraform-state-content pa-3">{{ formattedState }}</pre>
    <div v-else-if="error" class="text-center">{{ error.message }}</div>
</template>

<style scoped>
.terraform-state-content {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: var(--taskexec-bg);
  color: var(--taskexec-text);
  border: 1px solid var(--taskexec-border);
  border-radius: 12px;
  overflow: auto;
  font-size: 13px;
  max-height: 400px;
  margin-top: 8px;
}
</style>

<script>
import axios from 'axios';

export default {
  props: {
    projectId: Number,
    inventoryId: Number,
    stateId: Number,
  },

  data() {
    return {
      state: null,
      error: null,
    };
  },

  computed: {
    formattedState() {
      try {
        return JSON.stringify(JSON.parse(this.state.state), null, 2);
      } catch {
        return this.state.state;
      }
    },
  },

  async created() {
    try {
      this.state = (await axios.get(`/api/project/${this.projectId}/inventory/${this.inventoryId}/terraform/states/${this.stateId}`)).data;
    } catch (e) {
      if (e.response?.status === 404) {
        this.error = {
          message: this.$t('uiNoStateAvailable'),
        };
      } else {
        this.error = e;
      }
    }
  },
};
</script>
