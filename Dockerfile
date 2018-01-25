FROM    alpine
ADD     logchain /logchain
CMD     ["sh","-c","/logchain"]