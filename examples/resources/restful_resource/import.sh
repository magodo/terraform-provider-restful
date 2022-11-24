# The import spec consists of following keys:
#
# - id (Required)                   : The resource id.
# - path (Required)                 : The path used to create the resource.
# - update_path (Optional)          : The path used to update the resource.
# - delete_path (Optional)          : The path used to delete the resource.
# - query (Optional)                : The query parameters.
# - header (Optional)               : The header.
# - create_method (Optional)        : The method used to create the resource. Defaults to POST.
# - update_method (Optional)        : The method used to update the resource. Defaults to PUT
# - delete_method (Optional)        : The method used to delete the resource. Defaults to DELETE.
# - merge_patch_disabled (Optional) : Whether merge patch is disabled. Defaults to false.
# - body (Optional)                 : The interested properties in the response body that you want to manage via this resource. If you omit this, then all the properties will be keeping track, which in 
#                                     most cases is not what you want (e.g. the read only attributes shouldn't be managed).
#                                     The value of each property is not important here, hence leave them as `null`.
terraform import restful_resource.example '{
  "id": "/subscriptions/0-0-0-0/resourceGroups/example",
  "path": "/subscriptions/0-0-0-0/resourceGroups/example",
  "query": {"api-version": ["2020-06-01"]},
  "create_method": "PUT",
  "body": {
    "location": null,
    "tags": null
  }
}'
