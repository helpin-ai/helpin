# Helpin Form Tracking Event Payload Structure

This document provides an example of the nested JSON structure used for form tracking events in the Helpin JavaScript SDK.

## $form Event Payload

The `$form` event is triggered when a form is submitted. The form data is now stored in a nested JSON structure under `event_attributes` with form fields in a `fields` array.

```json
{
  "event_id": "",
  "user": {
    "anonymous_id": "1q5d25481s",
    "id": "user123",
    "email": "test@example.com",
    "custom": {},
    "company": {
      "id": "company123",
      "name": "Test Company",
      "created_at": "2023-01-01"
    }
  },
  "company": {
    "id": "company123",
    "name": "Test Company",
    "created_at": "2023-01-01"
  },
  "ids": {
    "fbp": "fb.0.1742799825963.414412939976805714"
  },
  "utc_time": "2025-06-18T08:14:23.105Z",
  "local_tz_offset": -300,
  "api_key": "test-api-key",
  "src": "helpin",
  "event_type": "$form",
  "namespace": "default",
  "event_attributes": {
    "form_id": "testForm",
    "form_action": "http://localhost:5173/examples/form-tracking.html",
    "form_method": "get",
    "fields": [
      {
        "tag": "input",
        "type": "text",
        "id": "name",
        "value": "Sheharyar khalid",
        "class": "",
        "name": "name",
        "attributes": {}
      },
      {
        "tag": "input",
        "type": "email",
        "id": "email",
        "value": "admin@mail.com",
        "class": "",
        "name": "email",
        "attributes": {}
      },
      {
        "tag": "input",
        "type": "password",
        "id": "password",
        "value": "",
        "class": "",
        "name": "password",
        "attributes": {}
      },
      {
        "tag": "input",
        "type": "number",
        "id": "age",
        "value": "24",
        "class": "",
        "name": "age",
        "attributes": {}
      },
      {
        "tag": "select",
        "id": "country",
        "value": "au",
        "class": "",
        "name": "country",
        "attributes": {}
      },
      {
        "tag": "input",
        "type": "checkbox",
        "id": "newsletter",
        "value": "on",
        "class": "",
        "name": "newsletter",
        "attributes": {}
      },
      {
        "tag": "textarea",
        "id": "comments",
        "value": "Saving the tracking form object for future reference.",
        "class": "",
        "name": "comments",
        "attributes": {}
      }
    ]
  },
  "referer": "http://localhost:5173/examples/index.html",
  "url": "http://localhost:5173/examples/form-tracking.html",
  "page_title": "Helpin SDK Test",
  "doc_path": "/examples/form-tracking.html",
  "doc_host": "localhost",
  "doc_search": "",
  "screen_resolution": "1920x1200",
  "vp_size": "1850x1052",
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
  "user_language": "en-US",
  "doc_encoding": "UTF-8",
  "utm": {}
}
```

## $form_field_change Event Payload

The `$form_field_change` event is triggered when a form field value changes. The field data is now stored in a nested JSON structure under `event_attributes.field`.

```json
{
  "event_id": "",
  "user": {
    "anonymous_id": "603c604s07"
  },
  "company": {},
  "ids": {},
  "utc_time": "2025-06-18T07:22:41.917Z",
  "local_tz_offset": -300,
  "api_key": "UMaugVPOWz",
  "src": "helpin",
  "event_type": "$form_field_change",
  "namespace": "default",
  "event_attributes": {
    "form_id": "simpleForm",
    "form_name": "simple-test-form",
    "field": {
      "tag": "input",
      "type": "email",
      "id": "email",
      "name": "user_email",
      "value": "updated@example.com",
      "data_attributes": {
        "test": "email-field"
      }
    }
  },
  "referer": "",
  "url": "http://localhost:3000/test/e2e/form-tracking-test.html",
  "page_title": "Helpin Form Tracking Test",
  "doc_path": "/test/e2e/form-tracking-test.html",
  "doc_host": "localhost",
  "doc_search": "",
  "screen_resolution": "1280x720",
  "vp_size": "1280x720",
  "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.28 Safari/537.36",
  "user_language": "en-US",
  "doc_encoding": "UTF-8",
  "utm": {}
}
```

## Key Changes in Payload Structure

1. **Nested JSON Structure**: Form data is now stored in a nested structure under `event_attributes` instead of being flattened at the root level.

2. **Fields Array**: Form fields are now stored as an array of objects in `event_attributes.fields` instead of using flattened keys like `field_1_tag`, `field_1_value`, etc.

3. **Field Object**: For field change events, the field data is stored as a single object in `event_attributes.field` with properties like `tag`, `type`, `id`, `value`, etc.

4. **ClickHouse Compatibility**: This structure takes advantage of improved ClickHouse JSON functions, allowing for more efficient storage and querying of nested data structures.

## Benefits

- **Cleaner Data Structure**: The nested JSON structure is more intuitive and easier to understand.
- **Better ClickHouse Integration**: Takes advantage of improved ClickHouse JSON functions.
- **More Flexible**: Easier to add new properties or modify the structure without changing the database schema.
- **Reduced Duplication**: No need to repeat field properties in the event name.
