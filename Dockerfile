FROM    vikings/alpine:base
ADD     logchain /logchain
ENTRYPOINT     ["sh","-c","/logchain"]