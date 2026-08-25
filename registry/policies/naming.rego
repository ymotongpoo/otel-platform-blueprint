package before_resolution

import rego.v1

# New attributes defined in this registry must live under the internal
# namespace com.example.*. Attributes taken over from upstream registries are
# referenced with `ref` and have no `id`, so they are not checked here.
deny contains violation if {
	some group in input.groups
	some attr in group.attributes
	attr.id
	not startswith(attr.id, "com.example.")
	violation := {
		"id": "internal_namespace_only",
		"type": "semconv_attribute",
		"category": "naming",
		"group": group.id,
		"attr": attr.id,
	}
}
