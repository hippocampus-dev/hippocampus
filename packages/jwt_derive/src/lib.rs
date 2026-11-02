#[proc_macro_derive(Encode)]
pub fn derive_encode(item: proc_macro::TokenStream) -> proc_macro::TokenStream {
    let ast = syn::parse_macro_input!(item as syn::DeriveInput);
    let ident = &ast.ident;

    let generated = quote::quote! {
        impl jwt::Encode for #ident {}
    };

    generated.into()
}
