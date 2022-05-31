terraform {
  required_providers {
    exoscale = {
      source  = "exoscale/exoscale"
      version = ">=0.33.0"
    }
  }

  // "optional" attributes are experimental
  experiments = [module_variable_optional_attrs]
}