shopt -s nullglob
for f in ~/.secrets/*; do
  export $(basename ${f})=$(cat ${f})
done
shopt -u nullglob
