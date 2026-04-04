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
