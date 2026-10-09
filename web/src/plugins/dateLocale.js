import dayjs from 'dayjs';
import 'dayjs/locale/cs';
import 'dayjs/locale/de';
import 'dayjs/locale/es';
import 'dayjs/locale/fr';
import 'dayjs/locale/it';
import 'dayjs/locale/ja';
import 'dayjs/locale/ko';
import 'dayjs/locale/nl';
import 'dayjs/locale/pl';
import 'dayjs/locale/pt';
import 'dayjs/locale/pt-br';
import 'dayjs/locale/ru';
import 'dayjs/locale/zh-cn';
import 'dayjs/locale/zh-tw';
import { normalizeLocale } from '../lib/locale';

export default function setDateLocale(locale) {
  dayjs.locale(normalizeLocale(locale).replace('_', '-'));
}
