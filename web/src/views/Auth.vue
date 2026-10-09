<template>
  <div class="auth">
    <v-dialog v-model="loginHelpDialog" max-width="600">
      <v-card>
        <v-card-title>
          {{ $t('howToFixSigninIssues') }}
          <v-spacer></v-spacer>
          <v-btn icon @click="loginHelpDialog = false">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </v-card-title>
        <v-card-text>
          <p class="text-body-1">
            {{ $t('firstlyYouNeedAccessToTheServerWhereTaskExecRunni') }}
          </p>
          <p class="text-body-1">
            {{ $t('executeTheFollowingCommandOnTheServerToSeeExisting') }}
          </p>
          <v-alert dense text color="info" style="font-family: monospace">
            {{ $t('taskexecUserList') }}
          </v-alert>
          <p class="text-body-1">
            {{ $t('youCanChangePasswordOfExistingUser') }}
          </p>
          <v-alert dense text color="info" style="font-family: monospace">
            {{
              $t('taskexecUserChangebyloginLoginUser123Password', {
                makePasswordExample: makePasswordExample(),
              })
            }}
          </v-alert>
          <p class="text-body-1">
            {{ $t('orCreateNewAdminUser') }}
          </p>
          <v-alert dense text color="info" style="font-family: monospace">
            taskexec user add --admin --login user123 --name User123 --email user123@example.com
            --password {{ makePasswordExample() }}
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn color="blue darken-1" text @click="loginHelpDialog = false">
            {{ $t('close2') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <div class="auth__shell">
      <div class="auth__preferences">
        <InterfacePreferences
          :dark-mode="darkMode"
          test-id="auth"
          @toggle-theme="$emit('toggle-theme')"
          @select-language="$emit('select-language', $event)"
        />
      </div>
      <div class="auth__layout">
        <section class="auth__intro">
          <div class="auth__brand">
            <img src="favicon.svg?v=te-play-30" width="52" height="52" alt="" />
            <div>
              <strong>Task<b>Exec</b></strong>
              <span>{{ $t('brandTagline') }}</span>
            </div>
          </div>
          <div class="auth__intro-body">
            <div class="auth__eyebrow">{{ $t('automationWorkspace') }}</div>
            <h1>{{ $t('automationUnderControl') }}</h1>
            <p>{{ $t('automationWorkspaceHint') }}</p>
            <div class="auth__tools" :aria-label="$t('authToolsLabel')">
              <span>Ansible</span><span>Terraform</span><span>OpenTofu</span>
              <span>{{ $t('automationScripts') }}</span>
            </div>
          </div>
          <div class="auth__signature"><span></span>TaskExec</div>
        </section>

        <v-card class="auth__card" flat>
          <v-card-text>
            <v-form
              @submit.prevent
              ref="signInForm"
              lazy-validation
              v-model="signInFormValid"
              class="auth__form"
            >
              <div class="auth__form-icon"><v-icon>mdi-login-variant</v-icon></div>
              <h2 v-if="screen === 'verification'" class="auth__title">
                {{ $t('authVerificationTitle') }}
              </h2>

              <h2 v-else-if="screen === 'recovery'" class="auth__title">
                {{ $t('authRecoveryTitle') }}
              </h2>

              <template v-else>
                <h2 class="auth__title">{{ $t('welcomeBack') }}</h2>
                <p class="auth__subtitle">{{ $t('signInWorkspaceHint') }}</p>
              </template>

              <v-alert :value="signInError != null" color="error" style="margin-bottom: 20px"
                >{{ signInError }}
              </v-alert>

              <div v-if="screen === 'verification'">
                <div v-if="verificationMethod === 'totp'" class="text-center mb-4">
                  {{ $t(
                  'uiOpenTheTwoStepVerificationAppOnYourMobileDeviceToGetYourVerificationCode'
                ) }}
                </div>

                <v-otp-input v-model="verificationCode" length="6" @finish="verify()"></v-otp-input>

                <v-divider class="my-6" />

                <div class="text-center">
                  <a @click="signOut()" class="mr-6">{{ $t('uiReturnToLogin') }}</a>
                  <a
                    v-if="
                      verificationMethod === 'totp' &&
                      authMethods.totp &&
                      authMethods.totp.allow_recovery
                    "
                    @click="screen = 'recovery'"
                  >
                    {{ $t('uiUseRecoveryCode') }}
                  </a>

                </div>
              </div>

              <div v-else-if="screen === 'recovery'">
                <div class="text-center mb-2">
                  {{ $t('uiUseYourRecoveryCodeToRegainAccessToYourAccount') }}
                </div>

                <v-text-field
                  class="mt-6"
                  outlined
                  v-model="recoveryCode"
                  @keyup.enter.native="signIn"
                  :label="$t('uiRecoveryCode')"
                  :rules="[(v) => !!v || $t('uiRecoveryCodeIsRequired')]"
                  required
                />

                <div>
                  <v-btn style="width: 100%" color="primary" @click="recovery()">
                    {{ $t('uiSend') }}
                  </v-btn>
                </div>

                <div class="text-center pt-6">
                  <a @click="screen = 'verification'">{{ $t('uiReturnToVerification') }}</a>
                </div>
              </div>

              <div v-else>
                <v-btn-toggle
                  v-if="loginTabs.length > 1"
                  v-model="loginTab"
                  mandatory
                  borderless
                  class="mb-7 d-flex"
                  :background-color="$vuetify.theme.dark ? '#212121' : 'grey lighten-3'"
                >
                  <v-btn
                    v-for="tab in loginTabs"
                    :key="tab.id || 'local'"
                    small
                    class="flex-grow-1"
                  >
                    {{ tab.name }}
                  </v-btn>
                </v-btn-toggle>

                <div v-if="(activeLoginTab && activeLoginTab.ldap) || loginWithPassword">
                  <v-text-field
                    v-model="username"
                    v-bind:label="$t('username')"
                    :rules="[(v) => !!v || $t('username_required')]"
                    required
                    outlined
                    :disabled="signInProcess"
                    id="auth-username"
                    autocomplete="username"
                    data-testid="auth-username"
                  ></v-text-field>

                  <v-text-field
                    v-model="password"
                    :label="$t('password')"
                    :rules="[(v) => !!v || $t('password_required')]"
                    type="password"
                    required
                    outlined
                    autocomplete="current-password"
                    :disabled="signInProcess"
                    @keyup.enter.native="signIn"
                    id="auth-password"
                    data-testid="auth-password"
                  ></v-text-field>

                  <v-btn
                    large
                    color="primary"
                    @click="signIn"
                    :disabled="signInProcess"
                    block
                    rounded
                    data-testid="auth-signin"
                  >
                    {{ $t('signIn') }}
                  </v-btn>
                </div>

                <div
                  class="auth__divider"
                  v-if="
                    (loginWithPassword || ldapProviders.length > 0) &&
                    oidcProviders.length > 0
                  "
                >{{ $t('uiOr') }}</div>

                <v-btn
                  large
                  v-for="provider in oidcProviders"
                  :color="provider.color || 'secondary'"
                  dark
                  class="mt-3"
                  @click="oidcSignIn(provider.id)"
                  block
                  :key="provider.id"
                  rounded
                >
                  <v-icon left dark v-if="provider.icon"> mdi-{{ provider.icon }} </v-icon>

                  {{ provider.name }}
                </v-btn>

                <div class="text-center mt-6" v-if="loginWithPassword && false">
                  <a @click="loginHelpDialog = true">{{ $t('dontHaveAccountOrCantSignIn') }}</a>
                </div>
              </div>
            </v-form>
          </v-card-text>
        </v-card>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import EventBus from '@/event-bus';
import InterfacePreferences from '@/components/InterfacePreferences.vue';

export default {
  components: { InterfacePreferences },
  props: { darkMode: Boolean },
  data() {
    return {
      signInFormValid: false,
      signInError: null,
      signInProcess: false,

      password: null,
      username: null,

      loginHelpDialog: null,

      oidcProviders: [],
      loginWithPassword: null,
      authMethods: {},

      ldapProviders: [],
      loginTab: 0,

      screen: null,

      verificationCode: null,
      verificationMethod: null,
      recoveryCode: null,
    };
  },

  async created() {
    const { status, verificationMethod } = await this.getAuthenticationStatus();

    switch (status) {
      case 'authenticated':
        this.redirectAfterLogin();
        break;
      case 'unauthenticated':
        await this.loadLoginData();
        break;
      case 'unverified':
        this.screen = 'verification';
        this.verificationMethod = verificationMethod;
        await this.loadLoginData();
        break;
      default:
        throw new Error(`Unknown authentication status: ${status}`);
    }
  },

  computed: {

    loginTabs() {
      const tabs = [];
      if (this.loginWithPassword) {
        tabs.push({ id: null, name: this.$t('signIn') });
      }
      this.ldapProviders.forEach((p) => tabs.push({ id: p.id, name: p.name, ldap: true }));
      return tabs;
    },

    activeLoginTab() {
      return this.loginTabs[this.loginTab] || this.loginTabs[0];
    },
  },

  methods: {
    async loadLoginData() {
      await axios({
        method: 'get',
        url: '/api/auth/login',
        responseType: 'json',
      }).then((resp) => {
        this.oidcProviders = resp.data.oidc_providers;
        this.loginWithPassword = resp.data.login_with_password;
        this.authMethods = resp.data.auth_methods || {};
        this.ldapProviders = resp.data.ldap_providers || [];
      });
    },

    async recovery() {
      this.signInProcess = true;

      try {
        await axios({
          method: 'post',
          url: '/api/auth/recovery',
          responseType: 'json',
          data: {
            recovery_code: this.recoveryCode,
          },
        });

        const { location } = document;
        document.location = location;
      } catch (e) {
        this.signInError = getErrorMessage(e);
      } finally {
        this.signInProcess = false;
      }
    },

    async signOut() {
      try {
        await axios({
          method: 'post',
          url: '/api/auth/logout',
          responseType: 'json',
        });

        const { location } = document;
        document.location = location;
      } catch (e) {
        EventBus.$emit('i-snackbar', {
          color: 'error',
          text: getErrorMessage(e),
        });
      }
    },

    makePasswordExample() {
      let pwd = '';
      const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
      const charactersLength = characters.length;
      for (let i = 0; i < 10; i += 1) {
        pwd += characters.charAt(Math.floor(Math.random() * charactersLength));
      }
      return pwd;
    },

    async getAuthenticationStatus() {
      try {
        await axios({
          method: 'get',
          url: '/api/user',
          responseType: 'json',
        });
      } catch (err) {
        if (err.response.status === 401) {
          switch (err.response.data.error) {
            case 'TOTP_REQUIRED':
              return {
                status: 'unverified',
                verificationMethod: 'totp',
              };
            case 'EMAIL_OTP_REQUIRED':
              return {
                status: 'unverified',
                verificationMethod: 'email',
              };
            default:
              return { status: 'unauthenticated' };
          }
        }
        throw err;
      }

      return { status: 'authenticated' };
    },

    async verify() {
      this.signInError = null;

      if (!this.$refs.signInForm.validate()) {
        return;
      }

      this.signInProcess = true;

      try {
        await axios({
          method: 'post',
          url: '/api/auth/verify',
          responseType: 'json',
          data: {
            passcode: this.verificationCode,
          },
        });

        this.redirectAfterLogin();
      } catch (err) {
        this.signInError = getErrorMessage(err);
      } finally {
        this.signInProcess = false;
      }
    },

    async signIn() {
      this.signInError = null;

      if (!this.$refs.signInForm.validate()) {
        return;
      }

      this.signInProcess = true;
      try {
        const body = {
          auth: this.username,
          password: this.password,
        };
        if (this.activeLoginTab && this.activeLoginTab.ldap) {
          body.method = 'ldap';
          body.provider = this.activeLoginTab.id;
        } else if (this.loginTabs.length > 1) {
          // Internal tab explicitly requests password auth so a typo never
          // hits the LDAP directory (legacy single-form behavior).
          body.method = 'password';
        }

        await axios({
          method: 'post',
          url: '/api/auth/login',
          responseType: 'json',
          data: body,
        });

        this.redirectAfterLogin();
        // document.location = document.baseURI + window.location.search;
      } catch (err) {
        if (err.response.status === 401) {
          this.signInError = this.$t('incorrectUsrPwd');
        } else {
          this.signInError = getErrorMessage(err);
        }
      } finally {
        this.signInProcess = false;
      }
    },

    async oidcSignIn(provider) {
      const params = new URLSearchParams();
      const returnTo = this.$route.query.return;
      if (returnTo) {
        params.set('return', returnTo);
      }
      const qs = params.toString();
      const suffix = qs ? `?${qs}` : '';
      document.location = `${document.baseURI}api/auth/oidc/${provider}/login${suffix}`;
    },

    redirectAfterLogin() {
      const redirectTo = this.$route.query.return;
      let baseURI = document.baseURI;

      if (redirectTo) {
        if (baseURI.endsWith('/')) {
          baseURI = baseURI.substring(0, baseURI.length - 1);
        }

        document.location = baseURI + redirectTo;

        return;
      }

      document.location = document.baseURI + window.location.search;
    },
  },
};
</script>
