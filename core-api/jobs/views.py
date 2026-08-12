from rest_framework import viewsets
from rest_framework.permissions import IsAuthenticated

from .models import Heartbeat, Job
from .serializers import HeartbeatSerializer, JobDetailSerializer, JobSerializer


class JobViewSet(viewsets.ModelViewSet):
    queryset = Job.objects.select_related("owner").prefetch_related("heartbeats").all()
    permission_classes = [IsAuthenticated]

    def get_serializer_class(self):
        if self.action == "retrieve":
            return JobDetailSerializer
        return JobSerializer

    def perform_create(self, serializer):
        serializer.save(owner=self.request.user)


class HeartbeatViewSet(viewsets.ReadOnlyModelViewSet):
    """Read-only from the API's perspective — heartbeats are written by the
    ingestion service directly against Postgres for throughput."""

    queryset = Heartbeat.objects.select_related("job").all()
    serializer_class = HeartbeatSerializer
    permission_classes = [IsAuthenticated]

    def get_queryset(self):
        qs = super().get_queryset()
        job_id = self.request.query_params.get("job")
        if job_id:
            qs = qs.filter(job_id=job_id)
        return qs
