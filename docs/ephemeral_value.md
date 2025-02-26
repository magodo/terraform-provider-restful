---
page_title: "Write Only Attribute"
description: |-
  The usage of `ephemeral_body` in `restful_resource` and `restful_operation`.
---

# Intro

> Write-only arguments are managed resource attributes that are configured by practitioners but are not persisted to the Terraform plan or state artifacts. Write-only arguments are supported in Terraform 1.11 and later. Write-only arguments should be used to handle secret values that do not need to be persisted in Terraform state, such as passwords, API keys, etc. The provider is expected to be the terminal point for an ephemeral value, which should either use the value by making the appropriate change to the API or ignore the value. Write-only arguments can accept [ephemeral values](https://developer.hashicorp.com/terraform/language/resources/ephemeral) and are not required to be consistent between plan and apply operations.

(Repeated from [FW doc](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments))

## Before

Previously, the `restful_resource` has an attribute `write_only_attrs`:

> `write_only_attrs` (List of String) A list of paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attributes that are only settable, but won't be read in GET response.

This attribute only helps to make the terraform state roundtrip consistent, i.e. use the state values of those specified attributes *as if* the API has returned them. This is more of a workaround for those *write-only* attributes to make the API work well with Terraform, instead of a concern of security. In fact, those *write-only*, (probably) *sensitive* values are stored in the state.

## Now

We recommend users to use the new `ephemeral_body` over `write_only_attrs`:

> `ephemeral_body` (Dynamic) The ephemeral (write-only) properties of the resource. This will be merge-patched to the `body` to construct the actual request body.

This is a [write-only attribute](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments), which is a dynamic object same as `body`. The values set in this attribute won't be stored in the state.

To be more specific:

1. Since the `ephemeral_body` is *write-only*, it's always `null` in the state. 
2. The provider will make sure to remove the subset of the `ephemeral_body` from the `output` (i.e. the API response), if the API returns it.

The 2nd point implies that the users can use the `ephemeral_body` to *hide* some sensitive fields in the API, albeit the API accept and return them. 

# How it Works

This section talks about some of the design details about `ephemeral_body`.

## Validation

There is a validation added for `ephemeral_body`, to make sure it has no overlap with `body`. The defintion of two objects are disjointed is defined as below:

> They are disjointed in one of the following cases:
>  - Both are objects: Having different sets of keys, or the values of the common key are disjointed.
>  - Otherwise, the two json values are regarded jointed, including both values have different types, or different values.

NOTE: Since the `ephemeral_body` will be used to JSON merge patch to `body` to construct the effective request body (see below), the JSON array is not considered separately to tell disjoint.

## Merge with `body`

During `Create` and `Update`, the `ephemeral_body` will be used to [JSON merge patch](https://datatracker.ietf.org/doc/html/rfc7386) to `body` to construct the effective request body, if not null.

## Diff Detection

> Since write-only arguments have no prior values, user intent or value changes cannot be determined with a write-only argument alone.

The framework provided several suggestions, and we choose the following:

> Use the resource's private state to store secure hashes of write-only argument values, the provider will then use the hash to determine if a write-only argument value has changed in later Terraform runs.

To be specific, the following data structure is stored in the private state, keyed by `ephemeral_body`:

```json
"ephemeral_body": {
    "hash": <Base64 encoded sha256 hash of the "ephemeral_body">,
    "null": <Base64 encoded nullified "ephemeral_body">
}
```

`ephemeral_body.hash` is used to compare with the `ephemeral_body` specified in the config in every plan.

In case there is a change in the `ephemeral_body`, the provider will mark the `output` as *unknown*, triggering a plan diff. The plan output in this case might not be confusing, as it only implies the `output` is known after apply, but doesn't indicate which attribute causes this change. The provider will log a message: `"ephemeral_body" has changed` in this case, to make things a bit clearer. 

## Remov `ephemeral_body` from `output`

The `output` by defaults contain everything returned by the API response. In case the API returns the attributes specified in `ephemeral_body`, it is effectively a leak of these *sensitive* attributes to the state file.

To resolve this, the `output` will perform a set difference against `ephemeral_body` before setting to the state, i.e. `output \ ephemeral_body`. The definition of this operation is as below:

> Difference removes the subset rhs from the lhs.
> If both are objects, remove the subset of the same keyed value, recursively, until reach to
> a non-object value for either lhs or rhs (regardless of the values), that key will be removed
> from lhs. If this is the last key in lhs, it will be removed one level upwards.

E.g.

```
lhs = {"m": {"a": 2, "b": 3, "c": {"x": 1, "y": 2, "z": 3}}}
rhs = {"m": {"a": 1, "b": 2, "c": {"x": 2, "y": 3}}}
lhs \ rhs = {"m": {"c": {"z": 3}}}
```

In fact, we don't care about the value of the `ephemeral_body`, only the shape. Therefore, we store the shape of the `ephemeral_body` in the private state as `ephemeral_body.null`, which represents the *nullified* `ephemeral_body`.

The reason to store the nullified ephemeral body in the private state is because during `Read()`, we don't have access to the Terraform config, which means the state is the only source that we can retrieve the shape of `ephemeral_body` and remove it from `output`.

NOTE: During import, therer is no way to avoid the *sensitive* attributes being exposed from `output`, if any. To avoid it, remember to specifiy the `ephemeral_body` in the config and re-apply after importing the resource.

# Use with ephemeral `restful_resource`

Since *write-only* is one of the valid contexts that can reference the ephemeral values, `ephemeral_body` can be used to reference, any attribute exposed by the `output` of the *ephemeral* `restful_resource`.