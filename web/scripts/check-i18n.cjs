/* Check dictionaries and Vue templates before shipping untranslated interface text. */
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const compiler = require('vue-template-compiler');

const root = path.resolve(__dirname, '../src');
const errors = [];
const dictionaries = {};
for (const file of fs.readdirSync(path.join(root, 'lang'))) {
  if (file === 'index.js' || !file.endsWith('.js')) continue;
  const source = fs.readFileSync(path.join(root, 'lang', file), 'utf8');
  dictionaries[file.slice(0, -3)] = vm.runInNewContext(
    `(${source.replace('export default', '').replace(/;\s*$/, '')})`,
  );
}
const english = dictionaries.en;
const keys = Object.keys(english);
const placeholders = (text) => [...new Set(text.match(/\{[\w]+\}/g) || [])].sort().join(',');
for (const [locale, messages] of Object.entries(dictionaries)) {
  for (const key of keys) {
    const value = messages[key];
    if (typeof value !== 'string' || !value.trim()) {
      errors.push(`${locale}.${key}: missing translation`);
    } else if (placeholders(value) !== placeholders(english[key])) {
      errors.push(`${locale}.${key}: interpolation parameters differ`);
    } else if (/ZXQ|QXZ/.test(value)) {
      errors.push(`${locale}.${key}: unfinished translation`);
    }
  }
  for (const key of Object.keys(messages)) {
    if (!Object.hasOwn(english, key)) errors.push(`${locale}.${key}: unknown key`);
  }
}

// Brand names, formats and executable examples are intentionally language-neutral.
const neutral = new Set([
  'Task', 'Exec', 'TaskExec', 'Ansible', 'Terraform', 'OpenTofu', 'Telegram',
  'JSON', 'YAML', 'TOTP', 'LDAP', 'Docker', 'Cron', 'HashiCorp Vault',
  'AWS Secrets Manager', 'Azure Key Vault', 'Devolutions Server', 'git:', 'local:',
  'TASKEXEC_SCHEDULE_TIMEZONE', 'schedule.timezone', 'us-east-1',
  'https://my-vault.vault.azure.net', 'impishmd/taskexec:latest-job', 'backend.tf', 'default',
]);
function isNeutral(text) {
  return !/[a-zA-Z]{2}/.test(text) || neutral.has(text)
    || /^(mdi-|\$vuetify\.icons\.|--?\w|taskexec (?:runner|user) )/.test(text);
}
function files(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const filename = path.join(directory, entry.name);
    return entry.isDirectory() ? files(filename) : [filename];
  });
}
let templates = 0;
for (const file of files(root).filter((f) => /\.(vue|js)$/.test(f) && !f.includes('/lang/'))) {
  const source = fs.readFileSync(file, 'utf8');
  const relative = path.relative(root, file);
  for (const match of source.matchAll(/(?:\$tc?|\.t)\(\s*(['"])([^'"\n]+)\1/g)) {
    if (!Object.hasOwn(english, match[2])) errors.push(`${relative}: unknown translation ${match[2]}`);
  }
  if (!file.endsWith('.vue')) continue;
  const component = compiler.parseComponent(source);
  if (!component.template) continue;
  templates += 1;
  const compiled = compiler.compile(component.template.content);
  for (const error of compiled.errors) errors.push(`${relative}: ${error}`);
  const seen = new Set();
  function visit(node) {
    if (!node || seen.has(node)) return;
    seen.add(node);
    if (node.type === 3 && !node.isComment) {
      const text = node.text.replace(/\s+/g, ' ').trim();
      if (!isNeutral(text)) errors.push(`${relative}: untranslated text: ${text}`);
    }
    if (node.type === 2) {
      for (const token of node.tokens || []) {
        if (typeof token !== 'string') continue;
        const text = token.replace(/\s+/g, ' ').trim();
        if (!isNeutral(text)) errors.push(`${relative}: untranslated text fragment: ${text}`);
      }
    }
    if (node.type !== 1) return;
    for (const attr of node.attrsList || []) {
      if (['label', 'title', 'hint', 'placeholder', 'aria-label', 'alt', 'no-data-text', 'loading-text', 'no-results-text', 'text', 'save-button-text', 'cancel-button-text', 'yes-button-title', 'no-button-title', 'success-message', 'object-title'].includes(attr.name)
        && !isNeutral(attr.value)) errors.push(`${relative}: untranslated ${attr.name}: ${attr.value}`);
      if (node.tag === 'i18n' && attr.name === 'path' && !Object.hasOwn(english, attr.value)) {
        errors.push(`${relative}: unknown translation ${attr.value}`);
      }
    }
    (node.children || []).forEach(visit);
    Object.values(node.scopedSlots || {}).forEach(visit);
    (node.ifConditions || []).forEach((condition) => visit(condition.block));
  }
  visit(compiled.ast);
}
if (errors.length) {
  console.error(errors.join('\n'));
  process.exitCode = 1;
} else {
  console.log(`Translations OK: ${Object.keys(dictionaries).length} languages, ${keys.length} keys each; ${templates} Vue templates checked.`);
}
