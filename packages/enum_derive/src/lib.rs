#[proc_macro_derive(EnumLen)]
pub fn derive_len(item: proc_macro::TokenStream) -> proc_macro::TokenStream {
    let ast = syn::parse_macro_input!(item as syn::DeriveInput);
    let ident = &ast.ident;
    let len = match ast.data {
        syn::Data::Enum(e) => e.variants.len(),
        _ => {
            panic!()
        }
    };

    let generated = quote::quote! {
        impl #ident {
            fn len() -> usize {
                #len
            }
        }
    };

    generated.into()
}

#[proc_macro_derive(EnumIter)]
pub fn derive_iter(item: proc_macro::TokenStream) -> proc_macro::TokenStream {
    let ast = syn::parse_macro_input!(item as syn::DeriveInput);
    let ident = &ast.ident;
    let iter_ident =
        proc_macro2::Ident::new(&format!("{ident}Iter"), proc_macro2::Span::call_site());
    let vis = &ast.vis;
    let variants = match ast.data {
        syn::Data::Enum(e) => e.variants,
        _ => {
            panic!()
        }
    };

    let arms = variants
        .iter()
        .enumerate()
        .map(|(i, v)| {
            let variant = &v.ident;
            match v.fields {
                syn::Fields::Unit => quote::quote! { #i => #ident::#variant, },
                _ => panic!(),
            }
        })
        .collect::<Vec<proc_macro2::TokenStream>>();

    let generated = quote::quote! {
        #[derive(Default)]
        #vis struct #iter_ident {
            i: usize,
        }
        impl Iterator for #iter_ident {
            type Item = #ident;

            fn next(&mut self) -> Option<Self::Item> {
                let item = match self.i {
                    #(#arms)*
                    _ => return None,
                };
                self.i += 1;
                Some(item)
            }
        }

        impl #ident {
            fn iter() -> #iter_ident {
                #iter_ident::default()
            }
        }
    };

    generated.into()
}

#[proc_macro_derive(EnumToString)]
pub fn derive_to_string(item: proc_macro::TokenStream) -> proc_macro::TokenStream {
    let ast = syn::parse_macro_input!(item as syn::DeriveInput);
    let ident = &ast.ident;
    let variants = match ast.data {
        syn::Data::Enum(e) => e.variants,
        _ => {
            panic!()
        }
    };

    let arms = variants
        .iter()
        .map(|v| {
            let variant = &v.ident;
            let s = proc_macro2::Literal::string(&variant.to_string());
            match v.fields {
                syn::Fields::Unit => {
                    quote::quote! { #ident::#variant => #s.to_string(), }
                }
                _ => panic!(),
            }
        })
        .collect::<Vec<proc_macro2::TokenStream>>();

    let generated = quote::quote! {
        impl #ident {
            fn to_string(&self) -> String {
                match self {
                    #(#arms)*
                }
            }
        }
    };

    generated.into()
}
