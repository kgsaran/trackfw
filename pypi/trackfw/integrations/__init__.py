"""Canonical multi-CLI agents and skills integration engine."""

from .catalog import CATALOG_VERSION, load_catalog, plan_deployments
from .manager import IntegrationManager

__all__ = ["CATALOG_VERSION", "IntegrationManager", "load_catalog", "plan_deployments"]
