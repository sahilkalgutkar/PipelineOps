from rest_framework import serializers

from .models import Heartbeat, Job


class HeartbeatSerializer(serializers.ModelSerializer):
    class Meta:
        model = Heartbeat
        fields = ["id", "job", "status", "duration_ms", "metadata", "received_at"]
        read_only_fields = ["id", "received_at"]


class JobSerializer(serializers.ModelSerializer):
    computed_status = serializers.CharField(read_only=True)
    owner_username = serializers.CharField(source="owner.username", read_only=True)

    class Meta:
        model = Job
        fields = [
            "id",
            "name",
            "description",
            "schedule_description",
            "expected_interval_seconds",
            "grace_period_seconds",
            "owner",
            "owner_username",
            "is_active",
            "last_heartbeat_at",
            "last_heartbeat_status",
            "computed_status",
            "created_at",
            "updated_at",
        ]
        read_only_fields = [
            "id",
            "last_heartbeat_at",
            "last_heartbeat_status",
            "created_at",
            "updated_at",
        ]


class JobDetailSerializer(JobSerializer):
    recent_heartbeats = serializers.SerializerMethodField()

    class Meta(JobSerializer.Meta):
        fields = JobSerializer.Meta.fields + ["recent_heartbeats"]

    def get_recent_heartbeats(self, obj):
        qs = obj.heartbeats.all()[:100]
        return HeartbeatSerializer(qs, many=True).data
