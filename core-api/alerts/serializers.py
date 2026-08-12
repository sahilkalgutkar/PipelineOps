from rest_framework import serializers

from .models import AlertEvent, AlertRule


class AlertRuleSerializer(serializers.ModelSerializer):
    job_name = serializers.CharField(source="job.name", read_only=True)

    class Meta:
        model = AlertRule
        fields = ["id", "job", "job_name", "channel", "target", "is_active", "created_at"]
        read_only_fields = ["id", "created_at"]


class AlertEventSerializer(serializers.ModelSerializer):
    job_name = serializers.CharField(source="job.name", read_only=True)

    class Meta:
        model = AlertEvent
        fields = [
            "id",
            "job",
            "job_name",
            "rule",
            "reason",
            "status",
            "notified",
            "triggered_at",
            "resolved_at",
        ]
        read_only_fields = fields
