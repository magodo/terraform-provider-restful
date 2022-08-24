# Terraform Provider Restful

This is a general Terraform provider aims to work for any platform as long as it exposes a RESTful API.

The document of this provider is available on [Terraform Provider Registry](https://registry.terraform.io/providers/magodo/restful/latest/docs).

## Features

- Different authentication choices: HTTP auth (Basic, Bearer), API Key auth and OAuth2 (client credential, password credential).
- Support resources created via either `PUT` or `POST`
- Support resources created via either `PUT` or `PATCH`
- Support polling asynchronous operations
- Partial `body` tracking: only the specified properties of the resource in the `body` attribute is tracked for diffs
- `restful_operation` resource that supports arbitrary Restful API call (e.g. `POST`) on create/update

## Why

Given there already exists platform oriented, first-class providers, why do I create this? The reason is that most providers today are manually maintained, which means some latest features are likely not available in these first-class providers. For this case, `terraform-provider-restful` can be used as your escape hatch.

Another common use case is that the platform you are currently working on do not have a Terraform provider yet. In this case, you can use `terraform-provider-restful` to manage the resources for that platform.

## Requirement

`terraform-provider-restful` has following assumptions about the API:

- The API is expected to support the following HTTP methods:
    - `POST`/`POST`: create the resource
    - `GET`: read the resource
    - `PUT`/`PATCH`: update the resource
    - `DELETE`: remove the resource
- The API content type is `application/json`
- The resource should have a unique identifier (e.g. `/foos/foo1`).
    - For resource that is created via `PUT`, the identifier is the API endpoint (i.e. `/foos/foo1`).
    - For resource that is created via `POST` (the endpoint is `/foos`), the identifier is retrieved from the response of `POST`, either the whole identifier (i.e. `/foos/foo1`), or the last segment behind the API endpoint (i.e. `foo1`).

Regarding the users, as `terraform-provider-restful` is essentially just a terraform-wrapped API client, practitioners have to know the details of the API for the target platform quite well, e.g.:

- Wheter a resource is created via `PUT` or `POST`
- Wheter a resource is updated via `PUT` or `PATCH`
- Whether any query parameter is needed for CRUD
- For asynchronous operations, how to poll the result 
- For resources that are created via `POST`, how to identify the `id`/`name` from the response
- etc.
