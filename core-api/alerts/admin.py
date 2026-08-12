from django.contrib import admin

from .models import AlertEvent, AlertRule


@admin.register(AlertRule)
class AlertRuleAdmin(admin.ModelAdmin):
    list_display = ("job", "channel", "target", "is_active")
    list_filter = ("channel", "is_active")


@admin.register(AlertEvent)
class AlertEventAdmin(admin.ModelAdmin):
    list_display = ("job", "status", "notified", "triggered_at", "resolved_at")
    list_filter = ("status", "notified")
