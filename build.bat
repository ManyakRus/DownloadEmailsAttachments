set FileName=DownloadEmailsAttachments

go build main.go
del /Q %FileName%.exe
del /Q %FileName%.zip
ren main.exe %FileName%.exe
copy %FileName%.exe %FileName%_ready\%FileName%.exe
copy readme.txt %FileName%_ready\readme.txt
copy readme.md %FileName%_ready\readme.md

"C:\Program Files\7-Zip\7z.exe" a -tzip %FileName%_ready %FileName%_ready 