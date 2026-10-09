<!--
Modal dialog which contains slot "form" and two buttons ("Cancel" and "OK").
Should be used to wrap forms which need to be displayed in modal dialog.
Can use used in tandem with ItemFormBase.js. See KeyForm.vue for example.
-->
<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <v-dialog
    ref="dialog"
    v-model="dialog"
    :max-width="maxWidth || 400"
    persistent
    :fullscreen="expandable && fullscreen"
    :transition="false"
    :content-class="
      `item-dialog ${ expandable ? 'item-dialog--expandable' : ''}
      item-dialog--${position} ${contentClass || ''}`
    "
  >
    <v-card :data-testid="testId">
      <v-card-title>
        <div class="item-dialog__title-text">
          <slot name="title">
            <v-icon v-if="icon" :color="iconColor" class="mr-3">{{ icon }}</v-icon>
            {{ title }}
          </slot>
        </div>
        <div class="item-dialog__title-actions">
          <HelpToggle v-if="helpButton" />
          <v-btn icon @click="toggleFullscreen()" v-if="expandable"
            :aria-label="$t(fullscreen ? 'dialogCollapse' : 'dialogExpand')">
            <v-icon>mdi-arrow-{{ fullscreen ? 'collapse' : 'expand' }}</v-icon>
          </v-btn>
          <v-btn icon @click="close()" data-testid="editDialog-close" :aria-label="$t('close')">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </div>

      </v-card-title>

      <v-card-text
        :class="{
          'pb-0': !hideButtons,
          'pa-0': noBodyPaddings,
        }"
        :style="{
          minHeight: minContentHeight + 'px'
        }"
      >
        <slot
          name="form"
          :onSave="onSave"
          :onError="clearFlags"
          :needSave="needSave"
          :needReset="needReset"
          :needHelp="needHelp"
        ></slot>
      </v-card-text>

      <v-card-actions v-if="!hideButtons">
        <v-spacer></v-spacer>

        <v-btn
          color="primary"
          text
          @click="close()"
        >
          {{ cancelButtonText || $t('cancel') }}
        </v-btn>

        <v-btn
          color="primary"
          depressed
          @click="needSave = true"
          v-if="saveButtonText != null"
          data-testid="editDialog-save"
        >
          {{ saveButtonText }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
<style lang="scss">
  .item-dialog--top {
    align-self: flex-start;
  }

  .item-dialog > .v-card > .v-card__title {
    display: flex;
    flex-wrap: nowrap;
    align-items: flex-start;
    gap: 12px;
    white-space: normal;
    word-break: normal;
    overflow-wrap: anywhere;
    line-height: 1.4;
    padding-bottom: 20px !important;
  }
  .item-dialog__title-text { flex: 1; min-width: 0; }
  .item-dialog__title-actions {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: 4px;
    background: transparent;
    margin: -4px -8px 0 0;
  }

.KeyDialog.v-dialog,
.VariableGroupDialog.v-dialog {
  max-height: calc(100vh - 48px);
  max-height: calc(100dvh - 48px);
  overflow: hidden;
  > .v-card {
    display: flex;
    flex-direction: column;
    max-height: inherit;
    > .v-card__text {
      overflow-y: auto;
      min-height: 0 !important;
      scrollbar-width: thin;
      scrollbar-color: var(--taskexec-border) transparent;
      &::-webkit-scrollbar { width: 6px; }
      &::-webkit-scrollbar-track { background: transparent; }
      &::-webkit-scrollbar-thumb {
        background: var(--taskexec-border);
        border-radius: 6px;
      }
      &:hover, &:focus-within {
        scrollbar-color: var(--taskexec-muted) transparent;
        &::-webkit-scrollbar-thumb { background: var(--taskexec-muted); }
      }
    }
    > .v-card__title, > .v-card__actions { flex-shrink: 0; }
  }
}
</style>
<script>

import EventBus from '@/event-bus';
import HelpToggle from '@/components/HelpToggle.vue';
import HelpContext from '@/components/HelpContext';

export default {
  components: { HelpToggle },
  mixins: [HelpContext],
  props: {
    testId: String,
    contentClass: String,
    position: String,
    title: String,
    icon: String,
    iconColor: String,
    value: Boolean,
    maxWidth: Number,
    minContentHeight: Number,
    eventName: String,
    hideButtons: Boolean,
    dontCloseOnSave: Boolean,
    cancelButtonText: String,
    saveButtonText: String,
    expandable: Boolean,
    name: {
      type: String,
      default: 'Unnamed',
    },
    helpButton: Boolean,
    noBodyPaddings: Boolean,
    noEscape: Boolean,
  },

  data() {
    return {
      dialog: false,
      needSave: false,
      needReset: false,
      fullscreen: null,
    };
  },

  watch: {
    async dialog(val) {
      this.needReset = val;
      this.$emit('input', val);
      if (val) {
        window.addEventListener('keydown', this.handleEscape);
      } else {
        this.needHelp = false;
        window.removeEventListener('keydown', this.handleEscape);
      }
    },

    async value(val) {
      this.dialog = val;
    },

    fullscreen(val) {
      if (val) {
        localStorage.setItem(`EditDialog_${this.name}__fullscreen`, '1');
      } else {
        localStorage.removeItem(`EditDialog_${this.name}__fullscreen`);
      }
    },
  },

  created() {
    this.fullscreen = localStorage.getItem(`EditDialog_${this.name}__fullscreen`) === '1';
  },

  methods: {
    onSave(e) {
      if (this.dontCloseOnSave) {
        this.clearFlags();
        return;
      }

      this.close(e);
    },

    toggleFullscreen() {
      this.fullscreen = !this.fullscreen;
    },

    close(e) {
      this.dialog = false;

      this.clearFlags();
      if (e) {
        this.$emit('save', e);
        if (this.eventName) {
          EventBus.$emit(this.eventName, e);
        }
      }
      this.$emit('close');
    },

    clearFlags() {
      this.needSave = false;
      this.needReset = false;
    },

    /**
     * Returns true if this dialog is the topmost active overlay (dialog or menu).
     * Uses the same check as Vuetify's VDialog (activeZIndex vs getMaxZIndex),
     * so a nested dialog or an open dropdown menu makes the parent non-topmost.
     */
    isTopmost() {
      const dialog = this.$refs.dialog;
      if (!dialog || typeof dialog.getMaxZIndex !== 'function') {
        return true;
      }
      return dialog.activeZIndex >= dialog.getMaxZIndex();
    },

    handleEscape(ev) {
      if (ev.key !== 'Escape' || this.dialog === false || this.noEscape) {
        return;
      }

      // Only the topmost dialog must react to Escape. If this dialog opened
      // another dialog (or a menu), let that one handle the key.
      if (!this.isTopmost()) {
        return;
      }

      this.close();
    },
  },
};
</script>
