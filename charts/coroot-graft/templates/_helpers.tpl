{{- define "coroot-graft.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "coroot-graft.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "coroot-graft.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "coroot-graft.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "coroot-graft.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "coroot-graft.selectorLabels" -}}
app.kubernetes.io/name: {{ include "coroot-graft.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "coroot-graft.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "coroot-graft.configMapName" -}}
{{- default (include "coroot-graft.fullname" .) .Values.config.existingConfigMap -}}
{{- end -}}

{{- define "coroot-graft.secretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else if .Values.secrets.nameOverride -}}
{{- .Values.secrets.nameOverride -}}
{{- else -}}
{{- printf "%s-secrets" (include "coroot-graft.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "coroot-graft.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "coroot-graft.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "coroot-graft.volumeSource" -}}
{{- $volume := .volume -}}
{{- if eq $volume.type "persistentVolumeClaim" }}
persistentVolumeClaim:
  claimName: {{ required "storage.existingClaim is required for persistentVolumeClaim" $volume.existingClaim }}
{{- else if eq $volume.type "hostPath" }}
hostPath:
  path: {{ required "storage.hostPath is required for hostPath storage" $volume.hostPath | quote }}
{{- else }}
emptyDir: {}
{{- end }}
{{- end -}}
