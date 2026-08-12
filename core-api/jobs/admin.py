from django.contrib import admin

from .models import Heartbeat, Job


@admin.register(Job)
class JobAdmin(admin.ModelAdmin):
    list_display = (
        "name",
        "computed_status",
        "last_heartbeat_at",
        "expected_interval_seconds",
        "is_active",
        "owner",
    )
    list_filter = ("is_active", "last_heartbeat_status")
    search_fields = ("name", "description")


@admin.register(Heartbeat)
class HeartbeatAdmin(admin.ModelAdmin):
    list_display = ("job", "status", "duration_ms", "received_at")
    list_filter = ("status",)
    search_fields = ("job__name",)
