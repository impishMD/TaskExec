<template>
  <span>
    <!-- eslint-disable-next-line vuejs-accessibility/form-control-has-label -->
    <textarea
      style="position: absolute; left: -9999px; top: -9999px"
      ref="copy_to_clipboard_textarea"
    ></textarea>

    <v-btn icon @click="copy()" :large="large">
      <v-icon :color="color">mdi-content-copy</v-icon>
    </v-btn>
  </span>
</template>

<script>
import EventBus from '@/event-bus';

export default {
  props: {
    text: String,
    successMessage: {
      type: String,
      default: '',
    },
    large: Boolean,
    color: String,
  },
  methods: {
    copy() {
      try {
        const el = this.$refs.copy_to_clipboard_textarea;
        el.value = this.text;
        el.focus();
        el.select();
        const successful = document.execCommand('copy');

        if (!successful) {
          throw new Error('Fallback copy failed');
        }

        EventBus.$emit('i-snackbar', {
          color: 'success',
          text: this.successMessage || this.$t('copiedToClipboard'),
        });
      } catch (e) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: this.$t('clipboardCopyFailed'),
        });
      }
    },
  },
};
</script>
