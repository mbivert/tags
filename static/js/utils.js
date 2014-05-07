$(document).ready(function(){
/*	XXX if enable, accordion disable (useful bug?)
	$('.panel-collapse').collapse({
		toggle: false
	})
*/

	$('form').each(function () {
		$(this).submit(function() {
			$(this).find("input[name=content]").val(
				$(this).find("div[name=rcontent]").text())
			return true
		})
	})
})
