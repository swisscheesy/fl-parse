#!/bin/sh
# Fake Decomp: $1=imdlst dir, $2=query, $3=output txt path. Fails for tables named *broken*.
case "$2" in
  *broken*) echo "simulated decomp failure" >&2; exit 3 ;;
esac
case "$3" in
  *part_number.txt)     printf '123|A\nabc|B\n456|C\n' > "$3" ;;
  *colloquial_name.txt) printf '1|x|NAME\n2|x|\n' > "$3" ;;
  *)                    printf 'a|b\n' > "$3" ;;
esac
