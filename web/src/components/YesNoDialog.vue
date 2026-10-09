<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <v-dialog
    v-model="dialog"
    :max-width="maxWidth || 290"
    content-class="YesNoDialog"
    scrollable
  >
    <v-card>
      <v-card-title class="headline">{{ title }}</v-card-title>

      <v-card-text>
        <slot>{{ text }}</slot>
      </v-card-text>

      <v-card-actions>
        <v-btn
          v-if="!hideNoButton"
          color="primary"
          text
          @click="no()"
        >
          {{ noButtonTitle || $t('cancel') }}
        </v-btn>

        <v-btn
          color="primary"
          :text="!yesButtonFilled"
          :depressed="yesButtonFilled"
          @click="yes()"
        >
          {{ yesButtonTitle || $t('yes') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
<script>

export default {
  props: {
    value: Boolean,
    title: String,
    text: String,
    yesButtonTitle: String,
    yesButtonFilled: Boolean,
    noButtonTitle: String,
    hideNoButton: Boolean,
    maxWidth: Number,
  },

  data() {
    return {
      dialog: false,
    };
  },

  watch: {
    async dialog(val) {
      this.$emit('input', val);
    },

    async value(val) {
      this.dialog = val;
    },
  },

  methods: {
    async yes() {
      this.$emit('yes');
      this.dialog = false;
    },
    async no() {
      this.$emit('no');
      this.dialog = false;
    },
  },
};
</script>

<style lang="scss">
.YesNoDialog.v-dialog {
  .v-card__title {
    white-space: normal;
    word-break: normal;
    overflow-wrap: break-word;
    padding-left: 24px;
    padding-right: 24px;
  }

  .v-card__text {
    word-break: normal;
    overflow-wrap: break-word;
    line-height: 1.6;
    scrollbar-width: thin;
    scrollbar-color: var(--taskexec-border) transparent;
  }

  .v-card__actions {
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 8px;

    > .v-btn.v-btn.v-size--default {
      margin: 0;
      min-height: 38px;
      height: auto;
      max-width: 100%;
      padding: 10px 16px;
      white-space: normal;

      .v-btn__content { flex: 1 1 auto; }
    }
  }
}

@media (max-width: 480px) {
  .YesNoDialog .v-card__actions {
    flex-direction: column-reverse;
    align-items: stretch;
  }
}
</style>
