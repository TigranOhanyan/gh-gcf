FROM public.ecr.aws/lambda/provided:al2023

RUN dnf install -y git && dnf clean all

COPY bootstrap ${LAMBDA_RUNTIME_DIR}/
CMD ["bootstrap"]
