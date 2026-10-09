import Vue from 'vue';
import Vuetify from 'vuetify/lib';
import OpenTofuIcon from '@/components/OpenTofuIcon.vue';
import PulumiIcon from '@/components/PulumiIcon.vue';
import TerragruntIcon from '@/components/TerragruntIcon.vue';
import HashicorpVaultIcon from '@/components/HashicorpVaultIcon.vue';
import {
  cs, de, en, es, fr, it, ja, ko, nl, pl, pt, ru, zhHans, zhHant,
} from 'vuetify/lib/locale';
import DvlsIcon from '../components/DvlsIcon.vue';
import AwsSmIcon from '../components/AwsSmIcon.vue';
import AzureKvIcon from '../components/AzureKvIcon.vue';
import i18n from './i18';

Vue.use(Vuetify);

export default new Vuetify({
  lang: {
    current: i18n.locale,
    locales: {
      cs, de, en, es, fr, it, ja, ko, nl, pl, pt, pt_br: pt, ru, zh_cn: zhHans, zh_tw: zhHant,
    },
  },
  theme: {
    dark: localStorage.getItem('darkMode') === '1',
    themes: {
      light: {
        primary: '#187563',
        secondary: '#203c46',
        accent: '#ad711c',
        success: '#218263',
        error: '#c94b5b',
        warning: '#a56a17',
        info: '#387baf',
      },
      dark: {
        primary: '#83d9bb',
        secondary: '#29434e',
        accent: '#ebbc73',
        success: '#83d9bb',
        error: '#f1919d',
        warning: '#ebbc73',
        info: '#8cbde6',
      },
    },
  },
  icons: {
    values: {
      tofu: {
        component: OpenTofuIcon,
      },
      pulumi: {
        component: PulumiIcon,
      },
      terragrunt: {
        component: TerragruntIcon,
      },
      hashicorp_vault: {
        component: HashicorpVaultIcon,
      },
      dvls: {
        component: DvlsIcon,
      },
      aws_sm: {
        component: AwsSmIcon,
      },
      azure_kv: {
        component: AzureKvIcon,
      },
    },
  },
});
