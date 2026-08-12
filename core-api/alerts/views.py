from rest_framework import viewsets
from rest_framework.permissions import IsAuthenticated

from .models import AlertEvent, AlertRule
from .serializers import AlertEventSerializer, AlertRuleSerializer


class AlertRuleViewSet(viewsets.ModelViewSet):
    queryset = AlertRule.objects.select_related("job").all()
    serializer_class = AlertRuleSerializer
    permission_classes = [IsAuthenticated]


class AlertEventViewSet(viewsets.ReadOnlyModelViewSet):
    queryset = AlertEvent.objects.select_related("job", "rule").all()
    serializer_class = AlertEventSerializer
    permission_classes = [IsAuthenticated]

    def get_queryset(self):
        qs = super().get_queryset()
        job_id = self.request.query_params.get("job")
        status_param = self.request.query_params.get("status")
        if job_id:
            qs = qs.filter(job_id=job_id)
        if status_param:
            qs = qs.filter(status=status_param)
        return qs
