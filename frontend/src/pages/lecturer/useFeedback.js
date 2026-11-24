import { useState } from 'react'

// The error and success messages of a form, with a counter that makes <FormFeedback> announce a
// repeated message again (the same text twice is still a new event for a screen reader).
export function useFeedback(initialSuccess = '') {
  const [error, setErrorState] = useState('')
  const [success, setSuccessState] = useState(initialSuccess)
  const [revision, setRevision] = useState(0)

  function setError(message) {
    setErrorState(message)
    if (message) setRevision((value) => value + 1)
  }
  function setSuccess(message) {
    setSuccessState(message)
    if (message) setRevision((value) => value + 1)
  }

  return { error, success, revision, setError, setSuccess }
}
