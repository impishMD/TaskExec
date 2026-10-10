// Event descriptions are stored in English, including historical records.
// Translate their known message formats at render time, preserving user data.
const resources = {
  Repository: 'repository',
  Environment: 'activityVariableGroup',
  Inventory: 'inventory',
  'Access Key': 'accessKey',
  'Host config for': 'hostConfig',
  View: 'view',
  'Secret storage': 'activitySecretStorage',
  Template: 'template',
  Schedule: 'schedule',
  Project: 'project',
};

const actions = {
  created: 'activityCreated',
  updated: 'activityUpdated',
  deleted: 'activityDeleted',
  'description updated': 'activityDescriptionUpdated',
};

const statuses = {
  WAITING: 'status_waiting',
  STARTING: 'status_starting',
  WAITING_CONFIRMATION: 'status_waiting_confirmation',
  CONFIRMED: 'status_confirmed',
  REJECTED: 'status_rejected',
  RUNNING: 'running',
  SUCCESS: 'status_success',
  ERROR: 'status_failed',
  STOPPING: 'status_stopping',
  STOPPED: 'status_stopped',
};

export default function formatActivityDescription(event, translate) {
  const description = event.description || '';
  let match = description.match(/^Task ID (\d+) \(([\s\S]*)\)( finished with status)? ([A-Z_]+)$/);
  if (match && statuses[match[4]]) {
    return translate(match[3] ? 'activityTaskFinished' : 'activityTaskStatus', {
      id: match[1], name: match[2], status: translate(statuses[match[4]]),
    });
  }

  match = description.match(/^User ID (\d+) (added to|removed from) team$/);
  if (match) {
    return translate(match[2] === 'added to' ? 'activityUserAdded' : 'activityUserRemoved', { id: match[1] });
  }
  match = description.match(/^Changed role for User ID (\d+)$/);
  if (match) return translate('activityUserRoleChanged', { id: match[1] });

  match = description.match(/^(Template|Schedule) ID (\d+) (created|updated|deleted|description updated)$/);
  if (match) {
    return translate(actions[match[3]], { resource: `${translate(resources[match[1]])} #${match[2]}` });
  }
  match = description.match(/^Secret storage with ID (\d+) has been updated$/);
  if (match) {
    return translate('activityUpdated', { resource: `${translate('activitySecretStorage')} #${match[1]}` });
  }
  match = description.match(/^Secret storage ([\s\S]*) has been (created|updated|deleted)$/);
  if (match) {
    return translate(actions[match[2]], { resource: `${translate('activitySecretStorage')} «${match[1]}»` });
  }
  match = description.match(/^(Repository|Environment|Inventory|Access Key|Host config for|View) ([\s\S]*) (created|updated|deleted)$/);
  if (match) {
    return translate(actions[match[3]], { resource: `${translate(resources[match[1]])} «${match[2]}»` });
  }
  match = description.match(/^Project (created|updated|deleted)$/);
  if (match) return translate(actions[match[1]], { resource: translate('project') });

  // Imported/custom messages must remain readable even when their format is unknown.
  return description;
}
