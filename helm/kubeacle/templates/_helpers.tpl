{{/*
Expand the name of the chart.
*/}}
{{- define "kubeacle.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If the release name contains the chart name it will be
used as a full name.
*/}}
{{- define "kubeacle.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label (name + version).
*/}}
{{- define "kubeacle.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels shared by all resources in this chart.
*/}}
{{- define "kubeacle.labels" -}}
helm.sh/chart: {{ include "kubeacle.chart" . }}
{{ include "kubeacle.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (used by Service selectors and Deployment matchLabels).
*/}}
{{- define "kubeacle.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubeacle.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend component full name.
*/}}
{{- define "kubeacle.backend.fullname" -}}
{{- printf "%s-backend" (include "kubeacle.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Backend-specific labels.
*/}}
{{- define "kubeacle.backend.labels" -}}
{{ include "kubeacle.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend-specific selector labels.
*/}}
{{- define "kubeacle.backend.selectorLabels" -}}
{{ include "kubeacle.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend image tag (falls back to appVersion).
*/}}
{{- define "kubeacle.backend.image" -}}
{{- $tag := .Values.backend.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.backend.image.repository $tag }}
{{- end }}

{{/*
Frontend component full name.
*/}}
{{- define "kubeacle.frontend.fullname" -}}
{{- printf "%s-frontend" (include "kubeacle.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Frontend-specific labels.
*/}}
{{- define "kubeacle.frontend.labels" -}}
{{ include "kubeacle.labels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Frontend-specific selector labels.
*/}}
{{- define "kubeacle.frontend.selectorLabels" -}}
{{ include "kubeacle.selectorLabels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Frontend image tag (falls back to appVersion).
*/}}
{{- define "kubeacle.frontend.image" -}}
{{- $tag := .Values.frontend.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.frontend.image.repository $tag }}
{{- end }}

{{/*
Prometheus component full name.
*/}}
{{- define "kubeacle.prometheus.fullname" -}}
{{- printf "%s-prometheus" (include "kubeacle.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Prometheus-specific labels.
*/}}
{{- define "kubeacle.prometheus.labels" -}}
{{ include "kubeacle.labels" . }}
app.kubernetes.io/component: prometheus
{{- end }}

{{/*
Prometheus-specific selector labels.
*/}}
{{- define "kubeacle.prometheus.selectorLabels" -}}
{{ include "kubeacle.selectorLabels" . }}
app.kubernetes.io/component: prometheus
{{- end }}

{{/*
Prometheus image tag.
*/}}
{{- define "kubeacle.prometheus.image" -}}
{{- printf "%s:%s" .Values.prometheus.image.repository .Values.prometheus.image.tag }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "kubeacle.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kubeacle.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
