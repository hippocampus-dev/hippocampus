use rsa::pkcs8::DecodePrivateKey;

#[test]
fn ok() {
    let private_key =
        rsa::RsaPrivateKey::from_pkcs8_pem(include_str!("fixtures/rsa2048-private.pem")).unwrap();
    let signed = jwt::sign_with_rsa(
        &private_key,
        &jwt::Header {
            alg: jwt::Algorithm::RS256,
            jku: None,
            jwk: None,
            kid: None,
            x5u: None,
            x5c: None,
            x5t: None,
            x5t_s256: None,
            typ: Some("JWT".to_string()),
            cty: None,
            crit: None,
        },
        &jwt::Claims {
            iss: Some("someone".to_string()),
            sub: None,
            aud: None,
            exp: None,
            nbf: None,
            iat: None,
            jti: None,
        },
    )
    .unwrap();
    assert!(jwt::verify_with_rsa(&rsa::RsaPublicKey::from(private_key), signed).is_ok());
}

#[test]
fn err() {
    let private_key =
        rsa::RsaPrivateKey::from_pkcs8_pem(include_str!("fixtures/rsa2048-private.pem")).unwrap();
    let signed = jwt::sign_with_rsa(
        &private_key,
        &jwt::Header {
            alg: jwt::Algorithm::RS256,
            jku: None,
            jwk: None,
            kid: None,
            x5u: None,
            x5c: None,
            x5t: None,
            x5t_s256: None,
            typ: Some("JWT".to_string()),
            cty: None,
            crit: None,
        },
        &jwt::Claims {
            iss: Some("someone".to_string()),
            sub: None,
            aud: None,
            exp: None,
            nbf: None,
            iat: None,
            jti: None,
        },
    )
    .unwrap();
    let invalid_private_key =
        rsa::RsaPrivateKey::from_pkcs8_pem(include_str!("fixtures/invalid-rsa2048-private.pem"))
            .unwrap();
    assert!(jwt::verify_with_rsa(&rsa::RsaPublicKey::from(invalid_private_key), signed).is_err());
}
