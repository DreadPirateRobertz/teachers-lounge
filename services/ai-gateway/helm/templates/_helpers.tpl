{{/*
Expand the name of the chart.
*/}}
{{- define "ai-gateway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ai-gateway.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "ai-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Cloud SQL instance connection string: project:region:instance
*/}}
{{- define "ai-gateway.cloudSqlInstance" -}}
{{ .Values.gcp.project }}:{{ .Values.gcp.region }}:{{ .Values.gcp.cloudSqlInstance }}
{{- end }}

