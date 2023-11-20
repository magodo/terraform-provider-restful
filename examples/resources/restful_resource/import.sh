# The import spec consists of following keys:
#
# - id (Required)               : The resource id.
# - path (Required)             : The path used to create the resource (as this is force new)
# - query (Optional)            : The query parameters.
# - header (Optional)           : The header.
# - body (Optional)             : The interested properties in the response body that you want to manage via this resource.
#                                 If you omit this, then all the properties will be keeping track, which in most cases is 
#                                 not what you want (e.g. the read only attributes shouldn't be managed).
#                                 The value of each property is not important here, hence leave them as `null`.
terraform import restful_resource.example '{
  "id": "/subscriptions/0-0-0-0/resourceGroups/example",
  "path": "/subscriptions/0-0-0-0/resourceGroups/example",
  "query": {"api-version": ["2020-06-01"]},
  "body": {
    "location": null,
    "tags": null
  }
}'
