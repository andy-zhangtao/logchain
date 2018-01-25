FROM    alpine
ADD     logchain /logchain
ENTRYPOINT     ["sh","-c","/logchain"]