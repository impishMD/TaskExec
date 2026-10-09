{{/* Adapted for TaskExec from Semaphore UI Charts; see NOTICE and LICENSE. */}}
{{- define "taskexec.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "taskexec.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 58 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "taskexec.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 58 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 58 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- define "taskexec.selectorLabels" -}}
app.kubernetes.io/name: {{ include "taskexec.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: server
{{- end -}}
{{- define "taskexec.labels" -}}
{{ include "taskexec.selectorLabels" . }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.labels }}
{{ toYaml . }}
{{- end }}
{{- end -}}
{{- define "taskexec.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default (printf "v%s" .Chart.AppVersion) .Values.image.tag) -}}
{{- end -}}
{{- end -}}
{{- define "taskexec.claimName" -}}
{{- default (include "taskexec.fullname" .) .Values.persistence.existingClaim -}}
{{- end -}}
{{- define "taskexec.basePath" -}}
{{- if .Values.general.host -}}
{{- (urlParse .Values.general.host).path | trimSuffix "/" -}}
{{- end -}}
{{- end -}}
{{- define "taskexec.environment" -}}
TASKEXEC_INTERFACE: "0.0.0.0"
TASKEXEC_PORT: "3000"
TASKEXEC_CONFIG_PATH: "/var/lib/taskexec/config"
TASKEXEC_DB_PATH: "/var/lib/taskexec"
TASKEXEC_TMP_PATH: "/tmp/taskexec"
TASKEXEC_WEB_ROOT: {{ .Values.general.host | quote }}
TASKEXEC_PASSWORD_LOGIN_DISABLED: {{ .Values.general.passwordLoginDisable | quote }}
TASKEXEC_NON_ADMIN_CAN_CREATE_PROJECT: {{ .Values.general.nonAdminCanCreateProject | quote }}
TASKEXEC_FORWARDED_ENV_VARS: {{ .Values.config.forwarded_env_vars | toJson | quote }}
TASKEXEC_OIDC_PROVIDERS: {{ include "taskexec.oidcProviders" . | quote }}
TASKEXEC_ADMIN: {{ .Values.admin.username | quote }}
TASKEXEC_ADMIN_NAME: {{ .Values.admin.fullname | quote }}
TASKEXEC_ADMIN_EMAIL: {{ .Values.admin.email | quote }}
TASKEXEC_DB_DIALECT: {{ .Values.database.type | quote }}
TASKEXEC_DB_OPTIONS: {{ toJson .Values.database.options | quote }}
{{- if eq .Values.database.type "sqlite" }}
TASKEXEC_DB_HOST: "/var/lib/taskexec/database.sqlite"
{{- else }}
TASKEXEC_DB_HOST: {{ required "database.host is required for an external database" .Values.database.host | quote }}
TASKEXEC_DB_PORT: {{ default (ternary 5432 3306 (eq .Values.database.type "postgres")) .Values.database.port | quote }}
TASKEXEC_DB: {{ .Values.database.name | quote }}
{{- end }}
{{- end -}}

{{/* ServiceAccount and provider dictionary follow semaphoreui/charts. */}}
{{- define "taskexec.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "taskexec.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "taskexec.oidcProviders" -}}
{{- $providers := dict -}}
{{- if .Values.oidc.enable -}}
{{- range $id, $values := .Values.oidc.providers -}}
{{- $provider := dict "display_name" $id "return_via_state" true -}}
{{- range $key, $value := omit $values "existingSecret" "clientSecretKey" "clientIdKey" "endpoint" -}}
{{- $_ := set $provider ($key | snakecase) $value -}}
{{- end -}}
{{- with $values.endpoint -}}
{{- $endpoint := dict -}}
{{- range $key, $value := . -}}
{{- $_ := set $endpoint ($key | snakecase) $value -}}
{{- end -}}
{{- $_ := set $provider "endpoint" $endpoint -}}
{{- end -}}
{{- if not $values.redirectUrl -}}
{{- $_ := set $provider "redirect_url" (printf "%s/api/auth/oidc/%s/redirect" (trimSuffix "/" $.Values.general.host) $id) -}}
{{- end -}}
{{- $_ := set $provider "client_secret_file" (printf "/etc/taskexec/oidc/%s/client-secret" $id) -}}
{{- if $values.clientIdKey -}}
{{- $_ := set $provider "client_id_file" (printf "/etc/taskexec/oidc/%s/client-id" $id) -}}
{{- end -}}
{{- $_ := set $providers $id $provider -}}
{{- end -}}
{{- end -}}
{{- $providers | toJson -}}
{{- end -}}
