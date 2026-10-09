<template>
  <EditDialog
    v-model="dialog"
    :save-button-text="(
      itemId === 'new'
        ? (invitesEnabled ? $t('invite') : $t('uiLink'))
        : $t('save')
    )"
    :title="itemId === 'new' ? $t('newTeamMember') : $t('editTeamMember')"
    @save="onSave"
  >
    <template v-slot:form="{ onSave, onError, needSave, needReset }">
      <TeamMemberForm
        :project-id="projectId"
        :item-id="itemId"
        @save="onSave"
        @error="onError"
        :need-save="needSave"
        :need-reset="needReset"
        :invites-enabled="invitesEnabled"
        :invite-type="inviteType"
        :roles="roles"
      />
    </template>
  </EditDialog>

</template>

<style lang="scss">
</style>

<script>
import EditDialog from './EditDialog.vue';
import TeamMemberForm from './TeamMemberForm.vue';

export default {
  components: {
    EditDialog,
    TeamMemberForm,
  },

  props: {
    value: Boolean,
    projectId: Number,
    itemId: [String, Number],
    invitesEnabled: Boolean,
    inviteType: String,
    roles: Array,
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
    onSave(e) {
      this.$emit('save', e);
    },
  },
};
</script>
