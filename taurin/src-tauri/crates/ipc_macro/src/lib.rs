mod typescript;

#[proc_macro_attribute]
pub fn estringify(
    _attr: proc_macro::TokenStream,
    item: proc_macro::TokenStream,
) -> proc_macro::TokenStream {
    let mut ast = syn::parse_macro_input!(item as syn::ItemFn);

    if let syn::ReturnType::Type(rarrow, ref type_) = ast.sig.output {
        if let syn::Type::Path(syn::TypePath { ref path, .. }) = **type_ {
            if let Some(segment) = path.segments.last() {
                if segment.ident == "Result" {
                    if let syn::PathArguments::AngleBracketed(ref argument) = segment.arguments {
                        let ok = argument.args.first();
                        let err = argument.args.last();
                        if let (Some(ok), Some(_err)) = (ok, err) {
                            ast.sig.output = syn::parse_quote! { #rarrow Result<#ok, String> };

                            let body = ast.block.as_ref();
                            if ast.sig.asyncness.is_some() {
                                let body: syn::Block = syn::parse_quote! {{
                                    ipc::async_estringify(async move || {
                                        #body
                                    }).await
                                }};
                                *ast.block = body;
                            } else {
                                let body: syn::Block = syn::parse_quote! {{
                                    ipc::estringify(|| #body)
                                }};

                                *ast.block = body;
                            }
                        }
                    }
                }
            }
        }
    }

    let generated = quote::quote! {
        #ast
    };

    generated.into()
}

#[proc_macro_attribute]
pub fn export(
    attr: proc_macro::TokenStream,
    item: proc_macro::TokenStream,
) -> proc_macro::TokenStream {
    let file = syn::parse_macro_input!(attr as syn::LitStr);
    let ast = syn::parse_macro_input!(item as syn::Item);

    let definition = match typescript::generate_definition(&ast) {
        Ok(definition) => definition,
        Err(err) => {
            return err.to_compile_error().into();
        }
    };

    let generated = quote::quote! {
        #ast

        inventory::submit! {
            crate::Definition {
                file: #file,
                body: #definition,
            }
        }
    };

    generated.into()
}
