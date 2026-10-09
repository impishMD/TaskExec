{{- define "taskexec.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "taskexec.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 58 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "taskexec.name" .) | trunc 58 | trimSuffix "-" -}}
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
{{- if .Values.config.webRoot -}}
{{- (urlParse .Values.config.webRoot).path | trimSuffix "/" -}}
{{- end -}}
{{- end -}}
{{- define "taskexec.environment" -}}
TASKEXEC_INTERFACE: "0.0.0.0"
TASKEXEC_PORT: "3000"
TASKEXEC_CONFIG_PATH: "/var/lib/taskexec/config"
TASKEXEC_DB_PATH: "/var/lib/taskexec"
TASKEXEC_TMP_PATH: "/tmp/taskexec"
TASKEXEC_WEB_ROOT: {{ .Values.config.webRoot | quote }}
TASKEXEC_ADMIN: {{ .Values.config.admin.username | quote }}
TASKEXEC_ADMIN_NAME: {{ .Values.config.admin.name | quote }}
TASKEXEC_ADMIN_EMAIL: {{ .Values.config.admin.email | quote }}
TASKEXEC_DB_DIALECT: {{ .Values.config.database.dialect | quote }}
TASKEXEC_DB_OPTIONS: {{ toJson .Values.config.database.options | quote }}
{{- if eq .Values.config.database.dialect "sqlite" }}
TASKEXEC_DB_HOST: "/var/lib/taskexec/database.sqlite"
{{- else }}
TASKEXEC_DB_HOST: {{ required "config.database.host is required for an external database" .Values.config.database.host | quote }}
TASKEXEC_DB_PORT: {{ default (ternary 5432 3306 (eq .Values.config.database.dialect "postgres")) .Values.config.database.port | quote }}
TASKEXEC_DB: {{ .Values.config.database.name | quote }}
{{- end }}
{{- end -}}
