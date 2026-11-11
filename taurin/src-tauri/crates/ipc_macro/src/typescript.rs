fn types(type_: &syn::Type) -> String {
    match type_ {
        syn::Type::Tuple(tuple) => {
            format!(
                "[{}]",
                tuple
                    .elems
                    .iter()
                    .map(types)
                    .collect::<Vec<String>>()
                    .join(", ")
            )
        }
        syn::Type::Path(syn::TypePath { path, .. }) => {
            if path.segments.len() == 1 {
                let segment = path.segments.first().unwrap();
                match segment.ident.to_string().as_str() {
                    "bool" => "boolean".to_string(),
                    "char" => "string".to_string(),
                    "String" => "string".to_string(),
                    "i8" | "i16" | "i32" | "i64" | "i128" | "u8" | "u16" | "u32" | "u64"
                    | "u128" | "f32" | "f64" => "number".to_string(),
                    "Vec" => {
                        if let syn::PathArguments::AngleBracketed(ref arguments) = segment.arguments
                        {
                            if let Some(syn::GenericArgument::Type(inner_type)) =
                                arguments.args.first()
                            {
                                return format!("{}[]", types(inner_type));
                            }
                        }
                        "any[]".to_string()
                    }
                    "Option" => {
                        if let syn::PathArguments::AngleBracketed(ref arguments) = segment.arguments
                        {
                            if let Some(syn::GenericArgument::Type(inner_type)) =
                                arguments.args.first()
                            {
                                return format!("{} | null", types(inner_type));
                            }
                        }
                        "any".to_string()
                    }
                    other => other.to_string(),
                }
            } else {
                "any".to_string()
            }
        }
        _ => "any".to_string(),
    }
}

mod tag {
    fn has_attribute<S>(attrs: &[syn::Attribute], ident_name: S) -> bool
    where
        S: AsRef<str>,
    {
        attrs
            .iter()
            .filter(|attr| attr.path().is_ident("serde"))
            .find_map(|attr| {
                if let Ok(punctuated) = attr.parse_args_with(
                    syn::punctuated::Punctuated::<syn::Expr, syn::Token![,]>::parse_terminated,
                ) {
                    for expr in punctuated {
                        if find_attribute(&expr, ident_name.as_ref()) {
                            return Some(true);
                        }
                    }
                }
                attr.parse_args::<syn::Expr>()
                    .ok()
                    .map(|expr| find_attribute(&expr, ident_name.as_ref()))
            })
            .unwrap_or(false)
    }

    fn find_attribute<S>(expr: &syn::Expr, ident_name: S) -> bool
    where
        S: AsRef<str>,
    {
        match expr {
            syn::Expr::Path(path) => path.path.is_ident(ident_name.as_ref()),
            _ => false,
        }
    }

    pub(super) fn has_untagged(attrs: &[syn::Attribute]) -> bool {
        has_attribute(attrs, "untagged")
    }

    fn extract_attribute_value<S>(attrs: &[syn::Attribute], ident_name: S) -> Option<String>
    where
        S: AsRef<str>,
    {
        attrs
            .iter()
            .filter(|attr| attr.path().is_ident("serde"))
            .find_map(|attr| {
                if let Ok(punctuated) = attr.parse_args_with(
                    syn::punctuated::Punctuated::<syn::Expr, syn::Token![,]>::parse_terminated,
                ) {
                    for expr in punctuated {
                        let value = find_attribute_value(&expr, ident_name.as_ref());
                        if value.is_some() {
                            return value;
                        }
                    }
                }
                attr.parse_args::<syn::Expr>()
                    .ok()
                    .and_then(|expr| find_attribute_value(&expr, ident_name.as_ref()))
            })
    }

    fn find_attribute_value<S>(expr: &syn::Expr, ident_name: S) -> Option<String>
    where
        S: AsRef<str>,
    {
        match expr {
            syn::Expr::Assign(assign) => {
                if let syn::Expr::Path(path) = &*assign.left {
                    if path.path.is_ident(ident_name.as_ref()) {
                        if let syn::Expr::Lit(syn::ExprLit {
                            lit: syn::Lit::Str(lit_str),
                            ..
                        }) = &*assign.right
                        {
                            return Some(lit_str.value());
                        }
                    }
                }
                None
            }
            _ => None,
        }
    }

    pub(super) fn extract_tag_value(attrs: &[syn::Attribute]) -> Option<String> {
        extract_attribute_value(attrs, "tag")
    }

    pub(super) fn extract_rename_all_value(attrs: &[syn::Attribute]) -> Option<String> {
        extract_attribute_value(attrs, "rename_all")
    }

    pub(super) fn apply_rename_all(name: &str, rename_all: &str) -> String {
        match rename_all {
            "lowercase" => name.to_lowercase(),
            "UPPERCASE" => name.to_uppercase(),
            "camelCase" => {
                let mut result = String::new();
                let mut capitalize_next = false;
                for (i, c) in name.chars().enumerate() {
                    if c == '_' {
                        capitalize_next = true;
                    } else if capitalize_next {
                        result.push(c.to_ascii_uppercase());
                        capitalize_next = false;
                    } else if i == 0 {
                        result.push(c.to_ascii_lowercase());
                    } else {
                        result.push(c);
                    }
                }
                result
            }
            "snake_case" => {
                let mut result = String::new();
                for (i, c) in name.chars().enumerate() {
                    if i > 0 && c.is_uppercase() {
                        result.push('_');
                        result.push(c.to_ascii_lowercase());
                    } else {
                        result.push(c.to_ascii_lowercase());
                    }
                }
                result
            }
            "PascalCase" => {
                let mut result = String::new();
                let mut capitalize_next = true;
                for c in name.chars() {
                    if c == '_' {
                        capitalize_next = true;
                    } else if capitalize_next {
                        result.push(c.to_ascii_uppercase());
                        capitalize_next = false;
                    } else {
                        result.push(c);
                    }
                }
                result
            }
            "kebab-case" => {
                let mut result = String::new();
                for (i, c) in name.chars().enumerate() {
                    if i > 0 && c.is_uppercase() {
                        result.push('-');
                        result.push(c.to_ascii_lowercase());
                    } else {
                        result.push(c.to_ascii_lowercase());
                    }
                }
                result
            }
            "SCREAMING_SNAKE_CASE" => {
                let mut result = String::new();
                for (i, c) in name.chars().enumerate() {
                    if i > 0 && c.is_uppercase() {
                        result.push('_');
                        result.push(c);
                    } else {
                        result.push(c.to_ascii_uppercase());
                    }
                }
                result
            }
            _ => name.to_string(),
        }
    }
}

mod variant {
    use super::*;

    fn process_named_fields(fields_named: &syn::FieldsNamed) -> String {
        fields_named
            .named
            .iter()
            .map(|field| {
                let field_name = field
                    .ident
                    .as_ref()
                    .map(|ident| ident.to_string())
                    .unwrap_or_else(|| "unnamed".to_string());
                let type_name = types(&field.ty);
                format!("{}: {};", field_name, type_name)
            })
            .collect::<Vec<String>>()
            .join("\n")
    }

    fn process_unnamed_fields(fields_unnamed: &syn::FieldsUnnamed) -> (String, bool) {
        if fields_unnamed.unnamed.len() == 1 {
            let field = fields_unnamed.unnamed.first().unwrap();
            let type_name = types(&field.ty);
            (type_name, true)
        } else {
            let types_array = fields_unnamed
                .unnamed
                .iter()
                .map(|field| types(&field.ty))
                .collect::<Vec<String>>()
                .join(", ");
            (types_array, false)
        }
    }

    pub(super) fn format_untagged_variant(
        variant: &syn::Variant,
        _rename_all: Option<&str>,
    ) -> String {
        match &variant.fields {
            syn::Fields::Named(fields_named) => {
                if fields_named.named.len() == 1 {
                    let field = fields_named.named.first().unwrap();
                    types(&field.ty)
                } else {
                    fields_named
                        .named
                        .iter()
                        .map(|field| {
                            let field_name = field
                                .ident
                                .as_ref()
                                .map(|ident| ident.to_string())
                                .unwrap_or_else(|| "unnamed".to_string());
                            let type_name = types(&field.ty);
                            format!("{{ {}: {}; }}", field_name, type_name)
                        })
                        .collect::<Vec<String>>()
                        .join(" | ")
                }
            }
            syn::Fields::Unnamed(fields_unnamed) => {
                let (result, is_single) = process_unnamed_fields(fields_unnamed);
                if is_single {
                    result
                } else {
                    format!("[{}]", result)
                }
            }
            syn::Fields::Unit => "null".to_string(),
        }
    }

    pub(super) fn format_tagged_variant(
        variant: &syn::Variant,
        tag_name: &str,
        rename_all: Option<&str>,
    ) -> String {
        let variant_name = variant.ident.to_string();
        let transformed_name = if let Some(rename_style) = rename_all {
            tag::apply_rename_all(&variant_name, rename_style)
        } else {
            variant_name
        };

        let fields = match &variant.fields {
            syn::Fields::Named(fields_named) => process_named_fields(fields_named),
            syn::Fields::Unnamed(fields_unnamed) => {
                let (result, is_single) = process_unnamed_fields(fields_unnamed);
                if is_single {
                    format!("value: {};", result)
                } else {
                    format!("value: [{}];", result)
                }
            }
            syn::Fields::Unit => String::new(),
        };

        if fields.is_empty() {
            format!("{{ {}: \"{}\" }}", tag_name, transformed_name)
        } else {
            format!("{{ {}: \"{}\", {} }}", tag_name, transformed_name, fields)
        }
    }

    pub(super) fn format_regular_variant(
        variant: &syn::Variant,
        rename_all: Option<&str>,
    ) -> String {
        let variant_name = variant.ident.to_string();
        let transformed_name = if let Some(rename_style) = rename_all {
            tag::apply_rename_all(&variant_name, rename_style)
        } else {
            variant_name
        };

        match &variant.fields {
            syn::Fields::Named(fields_named) => {
                let fields = process_named_fields(fields_named);
                format!("{{ {}: {{ {} }} }}", transformed_name, fields)
            }
            syn::Fields::Unnamed(fields_unnamed) => {
                let (result, is_single) = process_unnamed_fields(fields_unnamed);
                if is_single {
                    format!("{{ {}: {} }}", transformed_name, result)
                } else {
                    format!("{{ {}: [{}] }}", transformed_name, result)
                }
            }
            syn::Fields::Unit => format!("\"{}\"", transformed_name),
        }
    }
}

pub fn generate_definition(ast: &syn::Item) -> Result<String, syn::Error> {
    match &ast {
        syn::Item::Struct(item_struct) => {
            let name = item_struct.ident.to_string();
            let rename_all = tag::extract_rename_all_value(&item_struct.attrs);

            let fields = item_struct
                .fields
                .iter()
                .map(|field| {
                    let field_name = field
                        .ident
                        .as_ref()
                        .map(|id| id.to_string())
                        .unwrap_or_else(|| "unnamed".to_string());
                    let transformed_name = if let Some(ref rename_style) = rename_all {
                        tag::apply_rename_all(&field_name, rename_style)
                    } else {
                        field_name
                    };

                    let type_name = types(&field.ty);
                    format!("  {}: {};", transformed_name, type_name)
                })
                .collect::<Vec<String>>()
                .join("\n");
            Ok(format!("export interface {} {{\n{}\n}}", name, fields))
        }
        syn::Item::Enum(item_enum) => {
            let name = item_enum.ident.to_string();
            let rename_all = tag::extract_rename_all_value(&item_enum.attrs);

            if tag::has_untagged(&item_enum.attrs) {
                let variants = item_enum
                    .variants
                    .iter()
                    .map(|variant| variant::format_untagged_variant(variant, rename_all.as_deref()))
                    .collect::<Vec<String>>()
                    .join(" | ");
                Ok(format!("export type {} = {};", name, variants))
            } else if let Some(tag_name) = tag::extract_tag_value(&item_enum.attrs) {
                let variants = item_enum
                    .variants
                    .iter()
                    .map(|variant| {
                        variant::format_tagged_variant(variant, &tag_name, rename_all.as_deref())
                    })
                    .collect::<Vec<String>>()
                    .join(" | ");
                Ok(format!("export type {} = {};", name, variants))
            } else {
                let variants = item_enum
                    .variants
                    .iter()
                    .map(|variant| variant::format_regular_variant(variant, rename_all.as_deref()))
                    .collect::<Vec<String>>()
                    .join(" | ");
                Ok(format!("export type {} = {};", name, variants))
            }
        }
        syn::Item::Type(item_type) => {
            let name = item_type.ident.to_string();
            let type_name = types(&item_type.ty);
            Ok(format!("export type {} = {};", name, type_name))
        }
        _ => Err(syn::Error::new_spanned(
            ast,
            "export can only be used on structs, enums or type aliases",
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_struct() {
        let code = r#"
            struct User {
                id: u32,
                name: String,
                active: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  id: number;
  name: string;
  active: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_option() {
        let code = r#"
            struct User {
                id: u32,
                name: String,
                email: Option<String>,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  id: number;
  name: string;
  email: string | null;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_vector() {
        let code = r#"
            struct User {
                id: u32,
                name: String,
                tags: Vec<String>,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  id: number;
  name: string;
  tags: string[];
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_nested() {
        let code = r#"
            struct User {
                id: u32,
                name: String,
                metadata: Option<Vec<String>>,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  id: number;
  name: string;
  metadata: string[] | null;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_tuple() {
        let code = r#"
            struct Point {
                coordinates: (f32, f32, f32),
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface Point {
  coordinates: [number, number, number];
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum() {
        let code = r#"
            enum Status {
                Active,
                Inactive,
                Pending,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Status = "Active" | "Inactive" | "Pending";"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_unnamed_enum() {
        let code = r#"
            enum Message {
                Text(String),
                Number(i32),
                Boolean(bool),
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { Text: string } | { Number: number } | { Boolean: boolean };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_named_enum() {
        let code = r#"
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { Text: { content: string;
sender: string; } } | { Image: { url: string;
size: number; } };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_lowercase() {
        let code = r#"
            #[serde(rename_all = "lowercase")]
            struct User {
                ID: u32,
                NAME: String,
                ACTIVE: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  id: number;
  name: string;
  active: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_uppercase() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE")]
            struct User {
                id: u32,
                name: String,
                active: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  ID: number;
  NAME: string;
  ACTIVE: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_camelcase() {
        let code = r#"
            #[serde(rename_all = "camelCase")]
            struct User {
                user_id: u32,
                user_name: String,
                is_active: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  userId: number;
  userName: string;
  isActive: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_snake_case() {
        let code = r#"
            #[serde(rename_all = "snake_case")]
            struct User {
                userId: u32,
                userName: String,
                isActive: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  user_id: number;
  user_name: string;
  is_active: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_pascalcase() {
        let code = r#"
            #[serde(rename_all = "PascalCase")]
            struct User {
                user_id: u32,
                user_name: String,
                is_active: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  UserId: number;
  UserName: string;
  IsActive: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_kebab_case() {
        let code = r#"
            #[serde(rename_all = "kebab-case")]
            struct User {
                userId: u32,
                userName: String,
                isActive: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  user-id: number;
  user-name: string;
  is-active: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_struct_rename_all_screaming_snake_case() {
        let code = r#"
            #[serde(rename_all = "SCREAMING_SNAKE_CASE")]
            struct User {
                userId: u32,
                userName: String,
                isActive: bool,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export interface User {
  USER_ID: number;
  USER_NAME: string;
  IS_ACTIVE: boolean;
}"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_rename_all() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE")]
            enum Status {
                Active,
                Inactive,
                Pending,
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Status = "ACTIVE" | "INACTIVE" | "PENDING";"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_unnamed_enum_rename_all() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE")]
            enum Message {
                Text(String),
                Number(i32),
                Boolean(bool),
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { TEXT: string } | { NUMBER: number } | { BOOLEAN: boolean };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_tagged() {
        let code = r#"
            #[serde(tag = "type")]
            enum Message {
                Text { content: String },
                Image { url: String },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { type: "Text", content: string; } | { type: "Image", url: string; };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_untagged() {
        let code = r#"
            #[serde(untagged)]
            enum Value {
                String(String),
                Number(f64),
                Boolean(bool),
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Value = string | number | boolean;"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_named_enum_rename_all() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE")]
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { TEXT: { content: string;
sender: string; } } | { IMAGE: { url: string;
size: number; } };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_tagged_rename_all() {
        let code = r#"
            #[serde(tag = "type")]
            #[serde(rename_all = "UPPERCASE")]
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { type: "TEXT", content: string;
sender: string; } | { type: "IMAGE", url: string;
size: number; };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_rename_all_tagged() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE")]
            #[serde(tag = "type")]
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { type: "TEXT", content: string;
sender: string; } | { type: "IMAGE", url: string;
size: number; };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_tagged_rename_all_inline() {
        let code = r#"
            #[serde(tag = "type", rename_all = "UPPERCASE")]
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { type: "TEXT", content: string;
sender: string; } | { type: "IMAGE", url: string;
size: number; };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_enum_rename_all_tagged_inline() {
        let code = r#"
            #[serde(rename_all = "UPPERCASE", tag = "type")]
            enum Message {
                Text { content: String, sender: String },
                Image { url: String, size: u32 },
            }
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type Message = { type: "TEXT", content: string;
sender: string; } | { type: "IMAGE", url: string;
size: number; };"#;

        assert_eq!(result, expected);
    }

    #[test]
    fn test_type_alias() {
        let code = r#"
            type UserId = u32;
        "#;

        let ast = syn::parse_str(code).unwrap();
        let result = generate_definition(&ast).unwrap();

        let expected = r#"export type UserId = number;"#;

        assert_eq!(result, expected);
    }
}
