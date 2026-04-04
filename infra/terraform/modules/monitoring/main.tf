# GKE Managed Prometheus is enabled in the GKE module.
# This module sets up Cloud Monitoring dashboards and alert policies.

resource "google_monitoring_notification_channel" "email" {
  project      = var.project_id
  display_name = "TeachersLounge Alerts"
  type         = "email"

  labels = {
    email_address = "alerts@teacherslounge.app"
  }
}

resource "google_monitoring_alert_policy" "high_cpu" {
  project      = var.project_id
  display_name = "${var.cluster_name} High CPU"
  combiner     = "OR"

  conditions {
    display_name = "GKE Container CPU > 80%"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/cpu/limit_utilization\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0.8
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_MEAN"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "1800s"
  }
}

resource "google_monitoring_alert_policy" "high_memory" {
  project      = var.project_id
  display_name = "${var.cluster_name} High Memory"
  combiner     = "OR"

  conditions {
    display_name = "GKE Container Memory > 85%"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/memory/limit_utilization\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0.85
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_MEAN"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "1800s"
  }
}

resource "google_monitoring_alert_policy" "pod_restart" {
  project      = var.project_id
  display_name = "${var.cluster_name} Pod Restarts"
  combiner     = "OR"

  conditions {
    display_name = "GKE Pod restart count > 3 in 10 min"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/restart_count\""
      comparison      = "COMPARISON_GT"
      threshold_value = 3
      duration        = "0s"

      aggregations {
        alignment_period   = "600s"
        per_series_aligner = "ALIGN_DELTA"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "1800s"
  }
}

# --- HPA-specific alerts ---

resource "google_monitoring_alert_policy" "cpu_saturation" {
  project      = var.project_id
  display_name = "${var.cluster_name} CPU Saturation (HPA ceiling)"
  combiner     = "OR"

  conditions {
    display_name = "GKE Container CPU > 90% for 5 min (pod may be at HPA max)"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/cpu/limit_utilization\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0.9
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_MEAN"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content   = "CPU utilization is above 90% for over 5 minutes. If the deployment is at HPA maxReplicas, it cannot scale further. Investigate whether maxReplicas needs to be raised or the service needs optimization."
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s"
  }
}

resource "google_monitoring_alert_policy" "memory_saturation" {
  project      = var.project_id
  display_name = "${var.cluster_name} Memory Saturation (OOM risk)"
  combiner     = "OR"

  conditions {
    display_name = "GKE Container Memory > 90% for 5 min"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/memory/limit_utilization\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0.9
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_MEAN"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content   = "Memory utilization is above 90% for over 5 minutes. OOMKill is imminent if this continues. Check if HPA is at max or if memory limits need to be raised."
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s"
  }
}

resource "google_monitoring_alert_policy" "pod_scaling_frequency" {
  project      = var.project_id
  display_name = "${var.cluster_name} Pod Scaling Thrashing"
  combiner     = "OR"

  conditions {
    display_name = "Pod restart count > 5 in 5 min (scaling thrashing)"

    condition_threshold {
      filter          = "resource.type = \"k8s_container\" AND metric.type = \"kubernetes.io/container/restart_count\""
      comparison      = "COMPARISON_GT"
      threshold_value = 5
      duration        = "0s"

      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_DELTA"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content   = "High restart rate detected, which may indicate HPA flapping (scaling up and down rapidly). Check HPA stabilization window settings and resource targets."
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s"
  }
}
