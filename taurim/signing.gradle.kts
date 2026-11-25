configure<com.android.build.api.dsl.ApplicationExtension> {
    lint {
        // lintVitalAnalyzeUniversalRelease crashes with "findFirCompiledSymbol only works on compiled declarations" in Release/taurim v5.23.0 and v5.24.0
        checkReleaseBuilds = false
    }
    signingConfigs {
        create("release") {
            val keystoreFile = System.getenv("ANDROID_KEYSTORE_FILE")
            if (keystoreFile != null) {
                storeFile = file(keystoreFile)
                storePassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("ANDROID_KEY_ALIAS")
                keyPassword = System.getenv("ANDROID_KEY_PASSWORD")
            }
        }
    }
    buildTypes {
        getByName("release") {
            if (System.getenv("ANDROID_KEYSTORE_FILE") != null) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }
}
