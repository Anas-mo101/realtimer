# version_info=$(mysql --version)
# if [[ "$version_info" == *"Maria"* ]]; then
#     include_dir=$(mariadb_config --include)
# else
#     include_dir=$(mysql_config --include)
# fi
# include_dir="-l C:\Users\anmo\Documents\vcpkg\installed\x64-windows\include\mysql"

export CGO_ENABLED="1"
export CGO_CFLAGS=-I"C:\Users\anmo\Documents\vcpkg\installed\x64-windows\include\mysql"

go build -v -buildmode=c-shared -o build/realtimer_requester.dll http.go
