{{- define "keda-redis-scaler.image" -}}
{{- $image := empty .Values.image.tag | ternary (merge .Values.image (dict "tag" .Chart.AppVersion)) .Values.image -}}
{{- include "common.images.image" ( dict "imageRoot" $image ) -}}
{{- end -}}


{{- define "keda-redis-scaler.imagePullSecrets" -}}
{{- include "common.images.pullSecrets" (dict "images" (list .Values.image) "global" .Values.global) -}}
{{- end -}}
