# kotlinx.serialization keeps generated serializers via its bundled rules.
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.**
-keep,includedescriptorclasses class org.opentick.android.**$$serializer { *; }
-keepclassmembers class org.opentick.android.** {
    *** Companion;
}
-keepclasseswithmembers class org.opentick.android.** {
    kotlinx.serialization.KSerializer serializer(...);
}
# OkHttp
-dontwarn okhttp3.internal.platform.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**
