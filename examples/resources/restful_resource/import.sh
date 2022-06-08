# The import spec consists of following keys:
#
# - id (Required)               : The resource id.
# - query (Optional)            : The query parameters.
# - create_method (Optional)    : The import process needs to determine the resource's `path`, which is derived from manipulation on the `id`, based on whether `create_method` is `PUT` or `POST`.
#                                 This is only needed when current configured `create_method` is not as expected, i.e. if you have configured the `create_method` in the provider block, then this is
#                                 not needed to set here.
# - body (Optional)             : The interested properties in the response body that you want to manage via this resource. If you omit this, then all the properties will be keeping track, which in 
#                                 most cases is not what you want (e.g. the read only attributes shouldn't be managed).
#                                 The value of each property is not important here, hence leave them as `null`.
terraform import restful_resource.example '{
  "id": "/subscriptions/0-0-0-0/resourceGroups/example",
  "query": {"api-version": ["2020-06-01"]},
  "create_method": "PUT",
  "body": {
    "location": null,
    "tags": null
  }
}'
