<template>
  <v-container
    fluid
    fill-height
    align-center
    justify-center
    class="pa-0"
  >
    <v-card
      class="pa-6"
      style="
        max-width: 520px;
        border-radius: 12px;
        background-color: var(--highlighted-card-bg-color);
      "
    >
      <v-card-title>{{ $t('uiAcceptInvitation') }}<v-spacer />
      </v-card-title>
      <v-card-text class="pb-0">
        <div v-if="state === 'processing'" class="text-center pt-6 pb-0">
          <v-progress-circular indeterminate color="primary" />
          <div class="mt-6">{{ $t('uiAcceptingInvitation') }}</div>
        </div>

        <v-alert v-else-if="state === 'success'" type="success" text>
          {{ $t('uiInvitationAcceptedYouNowHaveAccessToTheProject') }}
        </v-alert>

        <v-alert v-else type="error" text>
          {{ errorMessage }}
        </v-alert>
      </v-card-text>

      <v-card-actions>
        <v-spacer />

        <v-btn v-if="state === 'success'" color="primary" @click="goToProject">
          {{ $t('uiGoToProject') }}
        </v-btn>

        <div v-else-if="state === 'error'" >
          <v-btn text color="primary" @click="goToDashboard" :disabled="!token">
            {{ $t('uiGoToDashboard') }}
          </v-btn>
          <v-btn text color="primary" @click="retry" :disabled="!token">
            {{ $t('uiTryAgain') }}
          </v-btn>
        </div>
      </v-card-actions>
    </v-card>
  </v-container>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import delay from '@/lib/delay';

export default {
  name: 'AcceptInvite',
  props: {
    token: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      state: 'processing',
      errorMessage: null,
      projectId: null,
    };
  },
  async created() {
    await this.process();
  },
  methods: {

    async process() {
      this.state = 'processing';
      this.errorMessage = null;

      await delay(2000);

      try {
        const res = (await axios({
          method: 'post',
          url: '/api/invites/accept',
          responseType: 'json',
          data: { token: this.token },
        })).data;
        this.projectId = res.project_id;
        this.state = 'success';
      } catch (err) {
        this.state = 'error';
        this.errorMessage = getErrorMessage(err);
      }
    },

    retry() {
      this.process();
    },

    goToProject() {
      // this.$router.push(`/project/${this.projectId}`);
      let baseURI = document.baseURI;
      if (baseURI.endsWith('/')) {
        baseURI = baseURI.slice(0, -1);
      }
      document.location = `${baseURI}/project/${this.projectId}`;
    },

    goToDashboard() {
      document.location = document.baseURI;
    },
  },
};
</script>
